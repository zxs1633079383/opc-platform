package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/zlc-ai/opc-platform/pkg/gateway"
	"go.uber.org/zap"
)

const channelName = "telegram"

// Channel implements gateway.Channel for Telegram.
type Channel struct {
	config  *gateway.TelegramConfig
	bot     *tgbotapi.BotAPI
	handler gateway.MessageHandler
	logger  *zap.SugaredLogger

	mu       sync.RWMutex
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup

	// Track allowed users/groups for access control.
	allowedUsers  map[string]bool
	allowedGroups map[string]bool
}

// New creates a new Telegram channel.
func New(config *gateway.TelegramConfig, logger *zap.SugaredLogger) (*Channel, error) {
	if config == nil {
		return nil, fmt.Errorf("telegram config is nil")
	}
	if config.Token == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}

	// Build allowed users/groups maps for O(1) lookup.
	allowedUsers := make(map[string]bool)
	for _, u := range config.AllowedUsers {
		allowedUsers[u] = true
	}
	allowedGroups := make(map[string]bool)
	for _, g := range config.AllowedGroups {
		allowedGroups[g] = true
	}

	return &Channel{
		config:        config,
		logger:        logger,
		allowedUsers:  allowedUsers,
		allowedGroups: allowedGroups,
	}, nil
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return channelName
}

// SetHandler sets the message handler.
func (c *Channel) SetHandler(handler gateway.MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// Start initializes the Telegram bot and starts polling for updates.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("telegram channel already running")
	}

	// Create bot API client.
	bot, err := tgbotapi.NewBotAPI(c.config.Token)
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}

	c.bot = bot
	c.stopChan = make(chan struct{})
	c.running = true

	c.logger.Infow("telegram bot authorized",
		"username", bot.Self.UserName,
		"id", bot.Self.ID,
	)

	// Start polling in a goroutine.
	c.wg.Add(1)
	go c.pollUpdates(ctx)

	return nil
}

// Stop gracefully stops the Telegram bot.
func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}

	close(c.stopChan)
	c.running = false
	c.mu.Unlock()

	// Wait for polling goroutine to finish.
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("telegram channel stopped gracefully")
	case <-ctx.Done():
		c.logger.Warn("telegram channel stop timed out")
	}

	return nil
}

// Send sends a message to a Telegram chat.
func (c *Channel) Send(ctx context.Context, resp *gateway.Response) error {
	c.mu.RLock()
	bot := c.bot
	c.mu.RUnlock()

	if bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}

	chatID, err := strconv.ParseInt(resp.ChannelID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID %q: %w", resp.ChannelID, err)
	}

	msg := tgbotapi.NewMessage(chatID, resp.Text)

	// Set parse mode based on format.
	switch resp.Format {
	case "markdown":
		msg.ParseMode = tgbotapi.ModeMarkdownV2
	case "html":
		msg.ParseMode = tgbotapi.ModeHTML
	default:
		msg.ParseMode = ""
	}

	// Set reply-to if specified.
	if resp.ReplyToID != "" {
		msgID, err := strconv.Atoi(resp.ReplyToID)
		if err == nil {
			msg.ReplyToMessageID = msgID
		}
	}

	// Split long messages (Telegram has a 4096 character limit).
	const maxLength = 4096
	if len(resp.Text) > maxLength {
		return c.sendLongMessage(ctx, chatID, resp.Text, msg.ParseMode, msg.ReplyToMessageID)
	}

	_, err = bot.Send(msg)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}

	return nil
}

// sendLongMessage splits a long message into multiple messages.
func (c *Channel) sendLongMessage(ctx context.Context, chatID int64, text string, parseMode string, replyTo int) error {
	const maxLength = 4096

	for len(text) > 0 {
		chunk := text
		if len(chunk) > maxLength {
			// Try to split at a newline.
			idx := strings.LastIndex(text[:maxLength], "\n")
			if idx > 0 {
				chunk = text[:idx]
			} else {
				chunk = text[:maxLength]
			}
		}

		msg := tgbotapi.NewMessage(chatID, chunk)
		msg.ParseMode = parseMode
		if replyTo != 0 {
			msg.ReplyToMessageID = replyTo
			replyTo = 0 // Only reply to the first message.
		}

		_, err := c.bot.Send(msg)
		if err != nil {
			return fmt.Errorf("send telegram message chunk: %w", err)
		}

		text = text[len(chunk):]

		// Small delay to avoid rate limiting.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	return nil
}

// pollUpdates continuously polls for Telegram updates.
func (c *Channel) pollUpdates(ctx context.Context) {
	defer c.wg.Done()

	timeout := c.config.PollingTimeout
	if timeout <= 0 {
		timeout = 60
	}

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = timeout

	updates := c.bot.GetUpdatesChan(updateConfig)

	for {
		select {
		case <-c.stopChan:
			c.bot.StopReceivingUpdates()
			return
		case <-ctx.Done():
			c.bot.StopReceivingUpdates()
			return
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			// Handle the message in a goroutine.
			go c.handleUpdate(ctx, &update)
		}
	}
}

// handleUpdate processes a single Telegram update.
func (c *Channel) handleUpdate(ctx context.Context, update *tgbotapi.Update) {
	msg := update.Message

	// Access control check.
	if !c.isAllowed(msg) {
		c.logger.Debugw("message from unauthorized user/group",
			"userId", msg.From.ID,
			"chatId", msg.Chat.ID,
		)
		return
	}

	// Skip empty messages.
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	// Handle command prefix if configured.
	prefix := c.config.CommandPrefix
	if prefix != "" {
		if !strings.HasPrefix(text, prefix) {
			return
		}
		text = strings.TrimPrefix(text, prefix)
		text = strings.TrimSpace(text)
	}

	// Build gateway message.
	gwMsg := &gateway.Message{
		ID:        strconv.Itoa(msg.MessageID),
		ChannelID: strconv.FormatInt(msg.Chat.ID, 10),
		Channel:   channelName,
		UserID:    strconv.FormatInt(msg.From.ID, 10),
		Username:  getUserName(msg.From),
		Text:      text,
		Metadata: map[string]string{
			"chatType":  msg.Chat.Type,
			"chatTitle": msg.Chat.Title,
		},
	}

	if msg.ReplyToMessage != nil {
		gwMsg.ReplyTo = strconv.Itoa(msg.ReplyToMessage.MessageID)
	}

	c.logger.Debugw("received telegram message",
		"messageId", gwMsg.ID,
		"chatId", gwMsg.ChannelID,
		"userId", gwMsg.UserID,
		"username", gwMsg.Username,
		"text", truncate(text, 50),
	)

	// Get handler.
	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()

	if handler == nil {
		c.logger.Warn("no message handler set, ignoring message")
		return
	}

	// Send typing indicator.
	typing := tgbotapi.NewChatAction(msg.Chat.ID, tgbotapi.ChatTyping)
	_, _ = c.bot.Send(typing)

	// Handle the message.
	resp, err := handler(ctx, gwMsg)
	if err != nil {
		c.logger.Errorw("message handler failed",
			"messageId", gwMsg.ID,
			"error", err,
		)

		// Send error response.
		errResp := &gateway.Response{
			ChannelID: gwMsg.ChannelID,
			ReplyToID: gwMsg.ID,
			Text:      fmt.Sprintf("❌ Error: %v", err),
		}
		_ = c.Send(ctx, errResp)
		return
	}

	// Send response if provided.
	if resp != nil && resp.Text != "" {
		resp.ChannelID = gwMsg.ChannelID
		if resp.ReplyToID == "" {
			resp.ReplyToID = gwMsg.ID
		}
		if err := c.Send(ctx, resp); err != nil {
			c.logger.Errorw("failed to send response",
				"messageId", gwMsg.ID,
				"error", err,
			)
		}
	}
}

// isAllowed checks if the message sender is allowed to use the bot.
func (c *Channel) isAllowed(msg *tgbotapi.Message) bool {
	// If no restrictions are configured, allow all.
	if len(c.allowedUsers) == 0 && len(c.allowedGroups) == 0 {
		return true
	}

	// Check user allowlist.
	if len(c.allowedUsers) > 0 {
		userID := strconv.FormatInt(msg.From.ID, 10)
		username := msg.From.UserName
		if c.allowedUsers[userID] || c.allowedUsers[username] {
			return true
		}
	}

	// Check group allowlist.
	if len(c.allowedGroups) > 0 {
		chatID := strconv.FormatInt(msg.Chat.ID, 10)
		if c.allowedGroups[chatID] {
			return true
		}
	}

	return false
}

// getUserName returns the best available name for a user.
func getUserName(user *tgbotapi.User) string {
	if user.UserName != "" {
		return "@" + user.UserName
	}
	name := user.FirstName
	if user.LastName != "" {
		name += " " + user.LastName
	}
	return name
}

// truncate truncates a string to the specified length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

package discord

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/zlc-ai/opc-platform/pkg/gateway"
	"go.uber.org/zap"
)

const channelName = "discord"

// Channel implements gateway.Channel for Discord.
type Channel struct {
	config  *gateway.DiscordConfig
	session *discordgo.Session
	handler gateway.MessageHandler
	logger  *zap.SugaredLogger

	mu       sync.RWMutex
	running  bool
	stopChan chan struct{}

	// Track allowed guilds/channels for access control.
	allowedGuilds   map[string]bool
	allowedChannels map[string]bool
}

// New creates a new Discord channel.
func New(config *gateway.DiscordConfig, logger *zap.SugaredLogger) (*Channel, error) {
	if config == nil {
		return nil, fmt.Errorf("discord config is nil")
	}
	if config.Token == "" {
		return nil, fmt.Errorf("discord bot token is required")
	}

	// Build allowed guilds/channels maps for O(1) lookup.
	allowedGuilds := make(map[string]bool)
	for _, g := range config.AllowedGuilds {
		allowedGuilds[g] = true
	}
	allowedChannels := make(map[string]bool)
	for _, c := range config.AllowedChannels {
		allowedChannels[c] = true
	}

	return &Channel{
		config:          config,
		logger:          logger,
		allowedGuilds:   allowedGuilds,
		allowedChannels: allowedChannels,
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

// Start initializes the Discord bot and starts listening for messages.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("discord channel already running")
	}

	// Create Discord session.
	session, err := discordgo.New("Bot " + c.config.Token)
	if err != nil {
		return fmt.Errorf("create discord session: %w", err)
	}

	// Set intents.
	session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	// Add message handler.
	session.AddHandler(c.handleMessage)

	// Open connection.
	if err := session.Open(); err != nil {
		return fmt.Errorf("open discord session: %w", err)
	}

	c.session = session
	c.stopChan = make(chan struct{})
	c.running = true

	c.logger.Infow("discord bot connected",
		"username", session.State.User.Username,
		"id", session.State.User.ID,
	)

	return nil
}

// Stop gracefully stops the Discord bot.
func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	close(c.stopChan)

	if c.session != nil {
		if err := c.session.Close(); err != nil {
			c.logger.Warnw("error closing discord session", "error", err)
		}
		c.session = nil
	}

	c.running = false
	c.logger.Info("discord channel stopped")

	return nil
}

// Send sends a message to a Discord channel.
func (c *Channel) Send(ctx context.Context, resp *gateway.Response) error {
	c.mu.RLock()
	session := c.session
	c.mu.RUnlock()

	if session == nil {
		return fmt.Errorf("discord session not initialized")
	}

	// Discord has a 2000 character limit.
	const maxLength = 2000

	text := resp.Text
	if len(text) > maxLength {
		return c.sendLongMessage(ctx, resp.ChannelID, text, resp.ReplyToID)
	}

	// Build message send params.
	data := &discordgo.MessageSend{
		Content: text,
	}

	// Set reply reference if specified.
	if resp.ReplyToID != "" {
		data.Reference = &discordgo.MessageReference{
			MessageID: resp.ReplyToID,
			ChannelID: resp.ChannelID,
		}
	}

	_, err := session.ChannelMessageSendComplex(resp.ChannelID, data)
	if err != nil {
		return fmt.Errorf("send discord message: %w", err)
	}

	return nil
}

// sendLongMessage splits a long message into multiple messages.
func (c *Channel) sendLongMessage(ctx context.Context, channelID string, text string, replyTo string) error {
	const maxLength = 2000

	first := true
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

		data := &discordgo.MessageSend{
			Content: chunk,
		}

		// Only reply to the first message.
		if first && replyTo != "" {
			data.Reference = &discordgo.MessageReference{
				MessageID: replyTo,
				ChannelID: channelID,
			}
			first = false
		}

		_, err := c.session.ChannelMessageSendComplex(channelID, data)
		if err != nil {
			return fmt.Errorf("send discord message chunk: %w", err)
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

// handleMessage processes incoming Discord messages.
func (c *Channel) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself.
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if we're still running.
	c.mu.RLock()
	if !c.running {
		c.mu.RUnlock()
		return
	}
	handler := c.handler
	c.mu.RUnlock()

	// Access control check.
	if !c.isAllowed(m.Message) {
		c.logger.Debugw("message from unauthorized guild/channel",
			"guildId", m.GuildID,
			"channelId", m.ChannelID,
		)
		return
	}

	// Get text content.
	text := strings.TrimSpace(m.Content)
	if text == "" {
		return
	}

	// Handle command prefix if configured.
	prefix := c.config.CommandPrefix
	if prefix != "" {
		// Also allow mentions as prefix.
		mentionPrefix := fmt.Sprintf("<@%s>", s.State.User.ID)
		mentionPrefixNick := fmt.Sprintf("<@!%s>", s.State.User.ID)

		if strings.HasPrefix(text, prefix) {
			text = strings.TrimPrefix(text, prefix)
		} else if strings.HasPrefix(text, mentionPrefix) {
			text = strings.TrimPrefix(text, mentionPrefix)
		} else if strings.HasPrefix(text, mentionPrefixNick) {
			text = strings.TrimPrefix(text, mentionPrefixNick)
		} else {
			return
		}
		text = strings.TrimSpace(text)
	}

	// Build gateway message.
	gwMsg := &gateway.Message{
		ID:        m.ID,
		ChannelID: m.ChannelID,
		Channel:   channelName,
		UserID:    m.Author.ID,
		Username:  getUserName(m.Author, m.Member),
		Text:      text,
		Metadata: map[string]string{
			"guildId":     m.GuildID,
			"guildName":   getGuildName(s, m.GuildID),
			"channelName": getChannelName(s, m.ChannelID),
		},
	}

	if m.MessageReference != nil {
		gwMsg.ReplyTo = m.MessageReference.MessageID
	}

	c.logger.Debugw("received discord message",
		"messageId", gwMsg.ID,
		"channelId", gwMsg.ChannelID,
		"userId", gwMsg.UserID,
		"username", gwMsg.Username,
		"text", truncate(text, 50),
	)

	if handler == nil {
		c.logger.Warn("no message handler set, ignoring message")
		return
	}

	// Show typing indicator.
	go func() {
		_ = s.ChannelTyping(m.ChannelID)
	}()

	// Handle the message.
	ctx := context.Background()
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

// isAllowed checks if the message is from an allowed guild/channel.
func (c *Channel) isAllowed(msg *discordgo.Message) bool {
	// If no restrictions are configured, allow all.
	if len(c.allowedGuilds) == 0 && len(c.allowedChannels) == 0 {
		return true
	}

	// Check guild allowlist.
	if len(c.allowedGuilds) > 0 && msg.GuildID != "" {
		if c.allowedGuilds[msg.GuildID] {
			return true
		}
	}

	// Check channel allowlist.
	if len(c.allowedChannels) > 0 {
		if c.allowedChannels[msg.ChannelID] {
			return true
		}
	}

	// Allow DMs if no specific restrictions.
	if msg.GuildID == "" && len(c.allowedGuilds) == 0 {
		return true
	}

	return false
}

// getUserName returns the best available name for a user.
func getUserName(user *discordgo.User, member *discordgo.Member) string {
	if member != nil && member.Nick != "" {
		return member.Nick
	}
	if user.GlobalName != "" {
		return user.GlobalName
	}
	return user.Username
}

// getGuildName returns the name of a guild.
func getGuildName(s *discordgo.Session, guildID string) string {
	if guildID == "" {
		return "DM"
	}
	guild, err := s.State.Guild(guildID)
	if err != nil {
		return guildID
	}
	return guild.Name
}

// getChannelName returns the name of a channel.
func getChannelName(s *discordgo.Session, channelID string) string {
	channel, err := s.State.Channel(channelID)
	if err != nil {
		return channelID
	}
	return channel.Name
}

// truncate truncates a string to the specified length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

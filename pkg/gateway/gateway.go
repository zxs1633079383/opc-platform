package gateway

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// Message represents an incoming message from any channel.
type Message struct {
	ID        string            `json:"id"`
	ChannelID string            `json:"channelId"`
	Channel   string            `json:"channel"` // "telegram", "discord", "slack", etc.
	UserID    string            `json:"userId"`
	Username  string            `json:"username"`
	Text      string            `json:"text"`
	ReplyTo   string            `json:"replyTo,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Response represents an outgoing response to a channel.
type Response struct {
	ChannelID string `json:"channelId"`
	ReplyToID string `json:"replyToId,omitempty"`
	Text      string `json:"text"`
	Format    string `json:"format,omitempty"` // "text", "markdown", "html"
}

// MessageHandler is called when a message is received from any channel.
type MessageHandler func(ctx context.Context, msg *Message) (*Response, error)

// Channel represents a communication channel (Telegram, Discord, etc.)
type Channel interface {
	// Name returns the channel type name (e.g., "telegram", "discord").
	Name() string

	// Start initializes and starts the channel.
	Start(ctx context.Context) error

	// Stop gracefully stops the channel.
	Stop(ctx context.Context) error

	// Send sends a message/response to the channel.
	Send(ctx context.Context, resp *Response) error

	// SetHandler sets the message handler for incoming messages.
	SetHandler(handler MessageHandler)
}

// Gateway manages multiple communication channels and routes messages to the dispatcher.
type Gateway struct {
	channels map[string]Channel
	handler  MessageHandler
	logger   *zap.SugaredLogger

	mu      sync.RWMutex
	running bool
}

// Config holds gateway configuration.
type Config struct {
	Telegram *TelegramConfig `yaml:"telegram,omitempty" json:"telegram,omitempty"`
	Discord  *DiscordConfig  `yaml:"discord,omitempty" json:"discord,omitempty"`
}

// TelegramConfig holds Telegram bot configuration.
type TelegramConfig struct {
	Token          string   `yaml:"token" json:"token"`
	AllowedUsers   []string `yaml:"allowedUsers,omitempty" json:"allowedUsers,omitempty"`
	AllowedGroups  []string `yaml:"allowedGroups,omitempty" json:"allowedGroups,omitempty"`
	CommandPrefix  string   `yaml:"commandPrefix,omitempty" json:"commandPrefix,omitempty"`
	WebhookURL     string   `yaml:"webhookUrl,omitempty" json:"webhookUrl,omitempty"`
	PollingTimeout int      `yaml:"pollingTimeout,omitempty" json:"pollingTimeout,omitempty"`
}

// DiscordConfig holds Discord bot configuration.
type DiscordConfig struct {
	Token           string   `yaml:"token" json:"token"`
	ApplicationID   string   `yaml:"applicationId" json:"applicationId"`
	AllowedGuilds   []string `yaml:"allowedGuilds,omitempty" json:"allowedGuilds,omitempty"`
	AllowedChannels []string `yaml:"allowedChannels,omitempty" json:"allowedChannels,omitempty"`
	CommandPrefix   string   `yaml:"commandPrefix,omitempty" json:"commandPrefix,omitempty"`
}

// New creates a new Gateway instance.
func New(logger *zap.SugaredLogger) *Gateway {
	return &Gateway{
		channels: make(map[string]Channel),
		logger:   logger,
	}
}

// SetHandler sets the global message handler for all channels.
func (g *Gateway) SetHandler(handler MessageHandler) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.handler = handler

	// Update handler on all registered channels.
	for _, ch := range g.channels {
		ch.SetHandler(handler)
	}
}

// RegisterChannel registers a communication channel.
func (g *Gateway) RegisterChannel(ch Channel) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	name := ch.Name()
	if _, exists := g.channels[name]; exists {
		return fmt.Errorf("channel %q already registered", name)
	}

	g.channels[name] = ch
	if g.handler != nil {
		ch.SetHandler(g.handler)
	}

	g.logger.Infow("channel registered", "channel", name)
	return nil
}

// Start starts all registered channels.
func (g *Gateway) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.running {
		return fmt.Errorf("gateway already running")
	}

	for name, ch := range g.channels {
		if err := ch.Start(ctx); err != nil {
			// Stop any channels that were started.
			for n, c := range g.channels {
				if n == name {
					break
				}
				_ = c.Stop(ctx)
			}
			return fmt.Errorf("start channel %q: %w", name, err)
		}
		g.logger.Infow("channel started", "channel", name)
	}

	g.running = true
	return nil
}

// Stop stops all registered channels.
func (g *Gateway) Stop(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.running {
		return nil
	}

	var errs []error
	for name, ch := range g.channels {
		if err := ch.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("stop channel %q: %w", name, err))
		} else {
			g.logger.Infow("channel stopped", "channel", name)
		}
	}

	g.running = false

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping channels: %v", errs)
	}
	return nil
}

// GetChannel returns a channel by name.
func (g *Gateway) GetChannel(name string) (Channel, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ch, ok := g.channels[name]
	return ch, ok
}

// Channels returns the names of all registered channels.
func (g *Gateway) Channels() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	names := make([]string, 0, len(g.channels))
	for name := range g.channels {
		names = append(names, name)
	}
	return names
}

// Send sends a response to a specific channel.
func (g *Gateway) Send(ctx context.Context, channelName string, resp *Response) error {
	g.mu.RLock()
	ch, ok := g.channels[channelName]
	g.mu.RUnlock()

	if !ok {
		return fmt.Errorf("channel %q not found", channelName)
	}

	return ch.Send(ctx, resp)
}

// Package ctfbot connects the bot to Discord.
package ctfbot

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

var statuses = []string{
	"Setting up your CTF events...",
	"Syncing with CTFTime's latest info...",
	"Retrieving event details from CTFTime...",
	"Preparing for your next CTF event...",
}

// NewClient returns a Discord client delivering events to listeners. Background
// work stops when ctx ends.
func NewClient(ctx context.Context, token string, listeners ...bot.EventListener) (*bot.Client, error) {
	client, err := disgo.New(token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(
			gateway.IntentGuilds, gateway.IntentGuildMessages,
			gateway.IntentGuildMessageReactions, gateway.IntentGuildScheduledEvents)),
		// Scheduled events are cached to tell real status changes from other edits.
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagGuilds, cache.FlagGuildScheduledEvents)),
		// Text from users and CTFtime ends up in messages: never let it ping @everyone.
		bot.WithRestConfigOpts(rest.WithDefaultAllowedMentions(discord.AllowedMentions{
			Parse: []discord.AllowedMentionType{discord.AllowedMentionTypeUsers, discord.AllowedMentionTypeRoles},
			Roles: []snowflake.ID{},
			Users: []snowflake.ID{},
		})),
		// One slow command, like /create, must not hold up every other event.
		bot.WithEventManagerConfigOpts(bot.WithAsyncEventsEnabled()),
		bot.WithEventListeners(listeners...),
		bot.WithEventListeners(readyListener(ctx)),
	)
	if err != nil {
		return nil, fmt.Errorf("create Discord client: %w", err)
	}
	return client, nil
}

// readyListener sets the presence on every Ready, which also fires on
// reconnections, and starts rotating it only once.
func readyListener(ctx context.Context) bot.EventListener {
	var rotation sync.Once
	return bot.NewListenerFunc(func(e *events.Ready) {
		log.Info("DiscordCTFHelper ready")
		setPresence(ctx, e.Client(), statuses[0])
		rotation.Do(func() { go rotatePresence(ctx, e.Client()) })
	})
}

func rotatePresence(ctx context.Context, client *bot.Client) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			setPresence(ctx, client, statuses[rand.IntN(len(statuses))]) //nolint:gosec // Not security sensitive.
		}
	}
}

func setPresence(ctx context.Context, client *bot.Client, status string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.SetPresence(ctx,
		gateway.WithOnlineStatus(discord.OnlineStatusOnline),
		gateway.WithListeningActivity(status),
	); err != nil {
		log.Error("failed to set presence", "err", err)
	}
}

// ResetCommands deletes the commands registered on the guild and the global ones.
func ResetCommands(client *bot.Client, guildID snowflake.ID) error {
	guildCommands, err := client.Rest.GetGuildCommands(client.ApplicationID, guildID, false)
	if err != nil {
		return fmt.Errorf("list guild commands: %w", err)
	}
	for _, cmd := range guildCommands {
		if err := client.Rest.DeleteGuildCommand(client.ApplicationID, guildID, cmd.ID()); err != nil {
			log.Error("failed to delete guild command", "command", cmd.Name(), "err", err)
		}
	}

	globalCommands, err := client.Rest.GetGlobalCommands(client.ApplicationID, false)
	if err != nil {
		return fmt.Errorf("list global commands: %w", err)
	}
	for _, cmd := range globalCommands {
		if err := client.Rest.DeleteGlobalCommand(client.ApplicationID, cmd.ID()); err != nil {
			log.Error("failed to delete global command", "command", cmd.Name(), "err", err)
		}
	}
	return nil
}

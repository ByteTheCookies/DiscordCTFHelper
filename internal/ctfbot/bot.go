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
		bot.WithEventListeners(readyListener(ctx), clearGuildCommands(ctx)),
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

// clearGuildCommands removes the guild-scoped commands of a server. They would
// otherwise show up next to the global ones, as duplicates. GuildReady fires for
// each server at startup, so this covers all of them.
func clearGuildCommands(parent context.Context) bot.EventListener {
	return bot.NewListenerFunc(func(e *events.GuildReady) {
		ctx, cancel := context.WithTimeout(parent, 10*time.Second)
		defer cancel()
		client := e.Client()
		// The empty slice matters: it has to reach Discord as [], not null.
		if _, err := client.Rest.SetGuildCommands(client.ApplicationID, e.GuildID, []discord.ApplicationCommandCreate{}, rest.WithCtx(ctx)); err != nil {
			log.Error("failed to clear guild commands", "guild_id", e.GuildID, "err", err)
		}
	})
}

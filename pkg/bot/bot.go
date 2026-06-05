package ctfbot

import (
	"config"
	"context"
	"fmt"
	"handlers"
	"time"

	"github.com/charmbracelet/log"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
)

var STATUSES = []string{
	"Setting up your CTF events...",
	"Syncing with CTFTime's latest info...",
	"Retrieving event details from CTFTime...",
	"Preparing for your next CTF event...",
}

func New(cfg config.Config, version string, commit string) *Bot {
	return &Bot{
		Cfg:     cfg,
		Version: version,
		Commit:  commit,
	}
}

type Bot struct {
	Cfg     config.Config
	Client  bot.Client
	Version string
	Commit  string
}

func (b *Bot) SetupBot(shouldCleanCommands *bool, listeners ...bot.EventListener) error {
	client, err := disgo.New(b.Cfg.Bot.Token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(
			gateway.IntentGuilds, gateway.IntentGuildMessages,
			gateway.IntentMessageContent, gateway.IntentGuildMembers,
			gateway.IntentGuildMessageReactions, gateway.IntentGuildScheduledEvents)),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagGuilds)),
		bot.WithEventListeners(listeners...),
		bot.WithEventListeners(handlers.CredsModalListener()),
		bot.WithEventListeners(handlers.RemoveHandler()),
		bot.WithEventListeners(handlers.VotePollHandler()),
		bot.WithEventListeners(handlers.JoinUserHandler()),
		bot.WithEventListeners(handlers.ReactionAddFeedMessageHandler()),
		bot.WithEventListeners(handlers.ReactionRemoveFeedMessageHandler()),
		bot.WithEventListeners(handlers.ScheduleEventUpdateHandler()),
	)
	if err != nil {
		return err
	}

	b.Client = *client

	if *shouldCleanCommands {
		for _, guildID := range b.Cfg.Bot.DevGuilds {
			log.Info("Clearing commands for guild", "guildID", guildID)
			if err := b.resetAllCommands(&guildID); err != nil {
				log.Error("failed to reset commands", "err", err)
			}
		}
	}

	return nil
}

func changePresenceStatus(_ context.Context, client bot.Client) error {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		status := STATUSES[time.Now().UnixNano()%int64(len(STATUSES))]
		err := client.SetPresence(
			ctx,
			gateway.WithOnlineStatus(discord.OnlineStatusOnline),
			gateway.WithListeningActivity(status),
		)
		if err != nil {
			log.Error("failed to change presence status", "err", err)
		}
		cancel()
	}

	return nil
}

// Delete all commands from Discord
func (b *Bot) resetAllCommands(guildID *snowflake.ID) error {
	if guildID != nil {
		fmt.Printf("Clearing guild commands for guild %d...\n", *guildID)
		commands, err := b.Client.Rest.GetGuildCommands(b.Client.ApplicationID, *guildID, false)
		if err != nil {
			return err
		}

		for _, cmd := range commands {
			if err := b.Client.Rest.DeleteGuildCommand(b.Client.ApplicationID, *guildID, cmd.ID()); err != nil {
				log.Printf("Error deleting guild command %s: %v\n", cmd.Name(), err)
			} else {
				fmt.Printf("✓ Deleted guild command: %s\n", cmd.Name())
			}
		}
	}

	fmt.Println("Clearing global commands...")
	commands, err := b.Client.Rest.GetGlobalCommands(b.Client.ApplicationID, false)
	if err != nil {
		return err
	}

	for _, cmd := range commands {
		if err := b.Client.Rest.DeleteGlobalCommand(b.Client.ApplicationID, cmd.ID()); err != nil {
			log.Printf("Error deleting global command %s: %v\n", cmd.Name(), err)
		} else {
			fmt.Printf("✓ Deleted global command: %s\n", cmd.Name())
		}
	}

	return nil
}

func (b *Bot) OnReady(_ *events.Ready) {
	log.Info("DiscordCTFHelper ready")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := b.Client.SetPresence(ctx, gateway.WithListeningActivity("Setting up your CTF events..."), gateway.WithOnlineStatus(discord.OnlineStatusOnline)); err != nil {
		log.Error("failed to set presence", "err", err)
	}

	// var id snowflake.ID = 1261228203251339374
	// if err := b.resetAllCommands(&id); err != nil {
	// 	log.Error("failed to reset commands", "err", err)
	// }

	log.Info("Starting presence status changer...")
	// Change presence every 10 minutes
	go changePresenceStatus(context.Background(), b.Client)
}

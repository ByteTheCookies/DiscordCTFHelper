// Command ctfhelper runs the Discord CTF helper bot.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/handler"
	dotenv "github.com/dotenv-org/godotenvvault"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/commands"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/config"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctf"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctfbot"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctftime"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/listeners"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Error("bot stopped", "err", err)
		os.Exit(1)
	}
}

func run() (err error) {
	syncCommands := flag.Bool("sync-commands", false, "Whether to sync commands to discord")
	cleanCommands := flag.Bool("clean-commands", false, "Whether to clean commands from discord")
	flag.Parse()

	if err := dotenv.Load(); err != nil {
		log.Warn("no .env file loaded, using the environment", "err", err)
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	setupLogger(cfg.Log)
	log.Info("Starting bot", "version", config.Version, "commit", config.Commit)

	st, err := store.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, st.Close()) }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ct := ctftime.New(ctftime.DefaultBaseURL)
	svc := ctf.NewService(st, ct)
	router := commands.New(st, ct, svc).Router(ctx)

	client, err := ctfbot.NewClient(ctx, cfg.Bot.Token, append(listeners.New(st, svc, cfg.GreetEmojiID).All(), router)...)
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client.Close(closeCtx)
	}()

	if *cleanCommands {
		for _, guildID := range cfg.Bot.DevGuilds {
			log.Info("Clearing commands", "guild_id", guildID)
			if err := ctfbot.ResetCommands(client, guildID); err != nil {
				log.Error("failed to reset commands", "err", err)
			}
		}
	}
	if *syncCommands {
		log.Info("Syncing commands", "guild_ids", cfg.Bot.DevGuilds)
		if err := handler.SyncCommands(client, commands.Commands, cfg.Bot.DevGuilds); err != nil {
			log.Error("failed to sync commands", "err", err)
		}
	}

	openCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.OpenGateway(openCtx); err != nil {
		return fmt.Errorf("open gateway: %w", err)
	}

	log.Info("Bot is running. Press CTRL-C to exit.")
	<-ctx.Done()
	log.Info("Shutting down bot...")
	return nil
}

func setupLogger(cfg config.LogConfig) {
	logger := log.NewWithOptions(os.Stdout, log.Options{
		Level:           cfg.Level,
		ReportCaller:    cfg.AddSource,
		ReportTimestamp: true,
		TimeFormat:      time.DateTime,
	})

	switch cfg.Format {
	case "json":
		logger.SetFormatter(log.JSONFormatter)
	case "text":
		logger.SetFormatter(log.TextFormatter)
	default:
		log.Error("Unknown log format", "format", cfg.Format)
	}

	log.SetDefault(logger)
	// disgo logs through slog: this gives its messages our level and format.
	slog.SetDefault(slog.New(logger))
}

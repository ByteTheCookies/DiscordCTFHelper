// Command ctfhelper runs the Discord CTF helper bot.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgo/rest"
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

	syncCommands(ctx, client)

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

// syncCommands publishes the commands globally, which makes them available in
// every server the bot is in or joins later. It's a single request whatever the
// number of servers, and commands that already exist don't count toward Discord's
// daily creation limit, so it's safe to run at each startup.
func syncCommands(ctx context.Context, client *bot.Client) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := handler.SyncCommands(client, commands.Commands, nil, rest.WithCtx(ctx)); err != nil {
		// The commands already published keep working, so this isn't fatal.
		log.Error("failed to sync commands", "err", err)
		return
	}
	log.Info("Commands synced", "count", len(commands.Commands))
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

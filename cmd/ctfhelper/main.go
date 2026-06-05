package main

import (
	"commands"
	"config"
	"context"
	"ctfbot"
	"database"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/handler"
	dotenv "github.com/dotenv-org/godotenvvault"
)

func init() {
	err := dotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file", "err", err)
	}
}

func main() {
	shouldSyncCommands := flag.Bool("sync-commands", false, "Whether to sync commands to discord")
	shouldCleanCommands := flag.Bool("clean-commands", false, "Whether to clean commands from discord")
	flag.Parse()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Error("failed to read config", "err", err)
		os.Exit(-1)
	}

	dbInstance := database.Setup()
	db, err := dbInstance.Connection().DB()
	if err != nil {
		log.Error("failed to get database instance", "err", err)
		os.Exit(-1)
	}
	defer db.Close()

	setupLogger(cfg.Log)
	log.Info("Starting bot-template...", "version", config.Version, "commit", config.Commit)
	log.Info("Syncing commands", "sync", *shouldSyncCommands)

	b := ctfbot.New(*cfg, config.Version, config.Commit)

	h := handler.New()

	// Command registrations
	h.Command("/version", commands.VersionHandler())
	h.Command("/create", commands.CreateHandler())
	h.Command("/remove", commands.RemoveHandler())
	h.Command("/flag", commands.FlagHandler())
	h.Command("/delete-flag", commands.DeleteFlagHandler())
	h.Command("/report", commands.ReportHandler())
	h.Command("/creds", commands.CredsHandler())
	h.Command("/delete-creds", commands.DeleteCredsHandler())
	h.Command("/next-ctfs", commands.NextCTFsHandler())
	h.Command("/init", commands.InitHandler())
	h.Command("/chall", commands.ChallHandler())
	h.Command("/vote", commands.VoteHandler())
	h.Command("/ping", commands.PingHandler())
	h.Command("/rules", commands.RulesHandler())
	h.Command("/help", commands.HelpHandler())
	h.Command("/guide", commands.GuideHandler())

	if err = b.SetupBot(shouldCleanCommands, h, bot.NewListenerFunc(b.OnReady)); err != nil {
		log.Error("failed to setup bot", "err", err)
		return
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		b.Client.Close(ctx)
	}()

	if *shouldSyncCommands {
		log.Info("Syncing commands", "guild_ids", cfg.Bot.DevGuilds)
		if err = handler.SyncCommands(&b.Client, commands.Commands, cfg.Bot.DevGuilds); err != nil {
			log.Error("failed to sync commands", "err", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = b.Client.OpenGateway(ctx); err != nil {
		log.Error("failed to open gateway", "err", err)
		return
	}

	log.Info("Bot is running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM)
	<-s
	log.Info("Shutting down bot...")
}

func setupLogger(cfg config.LogConfig) {
	opts := log.Options{
		Level:           cfg.Level,
		ReportCaller:    cfg.AddSource,
		ReportTimestamp: true,
		TimeFormat:      time.Kitchen,
	}

	logger := log.NewWithOptions(os.Stdout, opts)

	switch cfg.Format {
	case "json":
		logger.SetFormatter(log.JSONFormatter)
	case "text":
		logger.SetFormatter(log.TextFormatter)
	default:
		log.Error("Unknown log format", "format", cfg.Format)
	}

	log.SetDefault(logger)
}

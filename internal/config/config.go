// Package config reads the bot's configuration from the environment.
package config

import (
	"cmp"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/snowflake/v2"
)

// Version and Commit identify the build. They are set at build time with -ldflags.
var (
	Version = "dev"
	Commit  = "unknown"
)

// Config is the bot's configuration.
type Config struct {
	Log          LogConfig
	Bot          BotConfig
	DatabasePath string
	// GreetEmojiID is the custom emoji to greet new members with, 0 for the default one.
	GreetEmojiID snowflake.ID
}

// BotConfig configures the connection to Discord.
type BotConfig struct {
	DevGuilds []snowflake.ID
	Token     string
}

// LogConfig configures logging.
type LogConfig struct {
	Level     log.Level
	Format    string
	AddSource bool
}

// Load reads the configuration from the environment.
func Load() (Config, error) {
	level, err := log.ParseLevel(strings.ToUpper(os.Getenv("LOG_LEVEL")))
	if err != nil {
		return Config{}, fmt.Errorf("parse LOG_LEVEL: %w", err)
	}

	var guilds []snowflake.ID
	for s := range strings.SplitSeq(os.Getenv("GUILDS"), ",") {
		if id, err := snowflake.Parse(s); err == nil {
			guilds = append(guilds, id)
		}
	}

	var greetEmojiID snowflake.ID
	if s := os.Getenv("GREET_EMOJI_ID"); s != "" {
		greetEmojiID, err = snowflake.Parse(s)
		if err != nil {
			return Config{}, fmt.Errorf("parse GREET_EMOJI_ID: %w", err)
		}
	}

	return Config{
		Bot: BotConfig{DevGuilds: guilds, Token: os.Getenv("TOKEN")},
		Log: LogConfig{
			Level:     level,
			Format:    os.Getenv("LOG_FORMAT"),
			AddSource: os.Getenv("LOG_SOURCE") == "true",
		},
		DatabasePath: cmp.Or(os.Getenv("DB_PATH"), "data/ctfs.db"),
		GreetEmojiID: greetEmojiID,
	}, nil
}

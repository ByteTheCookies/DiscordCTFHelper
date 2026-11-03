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

// Version and Commit identify the build.
var (
	Version = "main"
	Commit  = "2.2.0"
)

// Config is the bot's configuration.
type Config struct {
	Log          LogConfig
	Bot          BotConfig
	DatabasePath string
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

	return Config{
		Bot: BotConfig{DevGuilds: guilds, Token: os.Getenv("TOKEN")},
		Log: LogConfig{
			Level:     level,
			Format:    os.Getenv("LOG_FORMAT"),
			AddSource: os.Getenv("LOG_SOURCE") == "true",
		},
		DatabasePath: cmp.Or(os.Getenv("DB_PATH"), "data/ctfs.db"),
	}, nil
}

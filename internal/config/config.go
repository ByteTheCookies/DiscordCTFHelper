package config

import (
	"os"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/disgoorg/snowflake/v2"
)

var (
	Version = "main"
	Commit  = "2.2.0"
)

func LoadConfig() (*Config, error) {
	level, err := log.ParseLevel(strings.ToUpper(os.Getenv("LOG_LEVEL")))
	if err != nil {
		return nil, err
	}

	cfg := Config{
		Bot: BotConfig{
			DevGuilds: func() []snowflake.ID {
				var ids []snowflake.ID
				for _, s := range strings.Split(os.Getenv("GUILDS"), ",") {
					if id, err := snowflake.Parse(s); err == nil {
						ids = append(ids, id)
					}
				}
				return ids
			}(),
			Token: os.Getenv("TOKEN"),
		},
		Log: LogConfig{
			Level:     level,
			Format:    os.Getenv("LOG_FORMAT"),
			AddSource: os.Getenv("LOG_SOURCE") == "true",
		},
	}

	return &cfg, nil
}

type Config struct {
	Log LogConfig `toml:"log"`
	Bot BotConfig `toml:"bot"`
}

type BotConfig struct {
	DevGuilds []snowflake.ID `toml:"dev_guilds"`
	Token     string         `toml:"token"`
}

type LogConfig struct {
	Level     log.Level `toml:"level"`
	Format    string    `toml:"format"`
	AddSource bool      `toml:"add_source"`
}

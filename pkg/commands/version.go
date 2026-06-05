package commands

import (
	"config"
	"fmt"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var version = discord.SlashCommandCreate{
	Name:        "version",
	Description: "Version command",
}

func VersionHandler() handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Version: %s\nCommit: %s", config.Version, config.Commit),
			Flags:   discord.MessageFlagEphemeral,
		})
	}
}

package commands

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var help = discord.SlashCommandCreate{
	Name:        "help",
	Description: "Get a list of available commands",
}

// helpText lists the commands the bot registers, wherever they were synced to.
func helpText() string {
	var sb strings.Builder
	sb.WriteString("# Available commands\n\n")
	for _, cmd := range Commands {
		c, ok := cmd.(discord.SlashCommandCreate)
		if !ok {
			continue
		}
		fmt.Fprintf(&sb, "**`/%s`** — %s\n", c.Name, c.Description)
		for _, opt := range c.Options {
			fmt.Fprintf(&sb, "- **`%s`**: %s\n", opt.OptionName(), opt.OptionDescription())
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func HelpHandler() handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		if err := e.DeferCreateMessage(true); err != nil {
			log.Error("failed to defer create message", "error", err)
			return err
		}

		_, err := e.CreateFollowupMessage(discord.MessageCreate{
			Embeds: []discord.Embed{{Description: helpText()}},
			Flags:  discord.MessageFlagEphemeral,
		})
		return err
	}
}

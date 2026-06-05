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

func HelpHandler() handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		if err := e.DeferCreateMessage(true); err != nil {
			log.Error("failed to defer create message", "error", err)
			return err
		}

		commands, err := e.Client().Rest.GetGlobalCommands(e.Client().ApplicationID, false)
		if err != nil {
			log.Error("failed to get global commands", "error", err)
			return err
		}

		var sb strings.Builder
		sb.WriteString("# Available commands\n\n")

		for _, cmd := range commands {
			switch c := cmd.(type) {
			case discord.SlashCommand:
				sb.WriteString(fmt.Sprintf("**`/%s`** — %s\n", c.Name(), c.Description))
				for _, opt := range c.Options {
					switch o := opt.(type) {
					case discord.ApplicationCommandOptionString:
						sb.WriteString(fmt.Sprintf("- **`%s`** (String): %s\n", o.Name, o.Description))
						if len(o.Choices) > 0 {
							sb.WriteString("  - *Choices*: ")
							for i, choice := range o.Choices {
								if i > 0 {
									sb.WriteString(", ")
								}
								sb.WriteString(fmt.Sprintf("`%s`", choice.Name))
							}
							sb.WriteString("\n")
						}
					case discord.ApplicationCommandOptionInt:
						sb.WriteString(fmt.Sprintf("- **`%s`** (Integer): %s\n", o.Name, o.Description))
						if o.MinValue != nil {
							sb.WriteString(fmt.Sprintf("  - *Min*: `%d`\n", *o.MinValue))
						}
						if o.MaxValue != nil {
							sb.WriteString(fmt.Sprintf("  - *Max*: `%d`\n", *o.MaxValue))
						}
						if len(o.Choices) > 0 {
							sb.WriteString("  - *Choices*: ")
							for i, choice := range o.Choices {
								if i > 0 {
									sb.WriteString(", ")
								}
								sb.WriteString(fmt.Sprintf("`%s`", choice.Name))
							}
							sb.WriteString("\n")
						}
					case discord.ApplicationCommandOptionBool:
						sb.WriteString(fmt.Sprintf("- **`%s`** (Boolean): %s\n", o.Name, o.Description))
					}
				}
			case discord.UserCommand:
				sb.WriteString(fmt.Sprintf("**User command**: `%s`\n", c.Name()))
			case discord.MessageCommand:
				sb.WriteString(fmt.Sprintf("**Message command**: `%s`\n", c.Name()))
			}
			sb.WriteString("\n")
		}

		_, err = e.CreateFollowupMessage(discord.MessageCreate{
			Content: sb.String(),

			Flags: discord.MessageFlagEphemeral,
		})
		return err
	}
}

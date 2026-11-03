package commands

import (
	"fmt"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/config"
)

var (
	versionCommand = discord.SlashCommandCreate{
		Name:        "version",
		Description: "Version command",
	}
	pingCommand = discord.SlashCommandCreate{
		Name:        "ping",
		Description: "Ping the bot to check if it's alive",
	}
	rulesCommand = discord.SlashCommandCreate{
		Name:        "rules",
		Description: "Get the rules for participating in CTF events",
	}
	guideCommand = discord.SlashCommandCreate{
		Name:        "guide",
		Description: "Guide users on how to participate in CTF events and use the bot effectively",
	}
	helpCommand = discord.SlashCommandCreate{
		Name:        "help",
		Description: "Get a list of available commands",
	}
)

func version(e *handler.CommandEvent) error {
	return e.CreateMessage(ephemeral(fmt.Sprintf("Version: %s\nCommit: %s", config.Version, config.Commit)))
}

func ping(e *handler.CommandEvent) error {
	return reply(e, fmt.Sprintf("Latency: %dms", e.Client().Gateway.Latency().Milliseconds()))
}

func rules(e *handler.CommandEvent) error {
	return reply(e, "### Here are the rules for participating in CTF events:\n\n"+
		"**1.** Be respectful to other participants and organizers.\n"+
		"**2.** Do not share flags or solutions outside the official CTF channels.\n"+
		"**3.** Do not use any unauthorized tools or techniques to solve challenges.\n"+
		"**4.** Follow the specific rules of each CTF event, as they may vary.\n"+
		"**5.** Have fun and learn from the experience!")
}

func guide(e *handler.CommandEvent) error {
	return reply(e, "## Welcome!\n"+
		"Before we start, run `/rules` to see all the guidelines for playing with us.\n\n"+
		"### How to Play\n"+
		"1. Check if any new CTFs have appeared in the **#feed** channel.\n"+
		"2. If you find one you’re interested in, simply **react** to it. You will be assigned a role and gain access to the channel dedicated to that CTF.\n\n"+
		"> You will receive a notification when the CTF starts.\n\n"+
		"### Useful Commands\n"+
		"- `/creds` — View or create CTF credentials\n"+
		"- `/chall` — Create a thread dedicated to a specific challenge\n"+
		"- `/flag` — Report that you’ve found a flag and share the achievement with everyone\n\n"+
		"*For more commands, run `/help`*")
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

func help(e *handler.CommandEvent) error {
	_, err := e.CreateFollowupMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{Description: helpText()}},
		Flags:  discord.MessageFlagEphemeral,
	})
	return err
}

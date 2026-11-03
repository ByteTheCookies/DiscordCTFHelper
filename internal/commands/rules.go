package commands

import (
	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var rules = discord.SlashCommandCreate{
	Name:        "rules",
	Description: "Get the rules for participating in CTF events",
}

func RulesHandler() handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		if err := e.DeferCreateMessage(true); err != nil {
			log.Error("failed to defer create message", "error", err)
			return err
		}

		response := "### Here are the rules for participating in CTF events:\n\n" +
			"**1.** Be respectful to other participants and organizers.\n" +
			"**2.** Do not share flags or solutions outside the official CTF channels.\n" +
			"**3.** Do not use any unauthorized tools or techniques to solve challenges.\n" +
			"**4.** Follow the specific rules of each CTF event, as they may vary.\n" +
			"**5.** Have fun and learn from the experience!"

		_, err := e.CreateFollowupMessage(discord.MessageCreate{
			Content: response,
			Flags:   discord.MessageFlagEphemeral,
		})
		return err
	}
}

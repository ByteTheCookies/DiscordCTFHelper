package commands

import (
	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var guide = discord.SlashCommandCreate{
	Name:        "guide",
	Description: "Guide users on how to partecipate in CTF events and use the bot effectively",
}

func GuideHandler() handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		if err := e.DeferCreateMessage(true); err != nil {
			log.Error("failed to defer create message", "error", err)
			return err
		}

		response := "## Welcome!\n" +
			"Before we start, run `/rules` to see all the guidelines for playing with us.\n\n" +
			"### How to Play\n" +
			"1. Check if any new CTFs have appeared in the **#feed** channel.\n" +
			"2. If you find one you’re interested in, simply **react** to it. You will be assigned a role and gain access to the channel dedicated to that CTF.\n\n" +
			"> You will receive a notification when the CTF starts.\n\n" +
			"### Useful Commands\n" +
			"- `/creds` — View or create CTF credentials\n" +
			"- `/chall` — Create a thread dedicated to a specific challenge\n" +
			"- `/flag` — Report that you’ve found a flag and share the achievement with everyone\n\n" +
			"*For more commands, run `/help`*"

		_, err := e.CreateFollowupMessage(discord.MessageCreate{
			Content: response,
			Flags:   discord.MessageFlagEphemeral,
		})
		return err
	}
}

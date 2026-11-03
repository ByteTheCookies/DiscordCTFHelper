package commands

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var ping = discord.SlashCommandCreate{
	Name:        "ping",
	Description: "Ping the bot to check if it's alive",
}

func PingHandler() handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		if err := e.DeferCreateMessage(true); err != nil {
			log.Error("failed to defer create message", "error", err)
			return err
		}

		latency := e.Client().Gateway.Latency()

		response := fmt.Sprintf("Latency: %dms", latency.Milliseconds())
		_, err := e.CreateFollowupMessage(discord.MessageCreate{
			Content: response,
		})
		return err
	}
}

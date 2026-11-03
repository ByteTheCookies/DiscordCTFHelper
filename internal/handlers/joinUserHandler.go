package handlers

import (
	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func JoinUserHandler() bot.EventListener {
	return bot.NewListenerFunc(func(e *events.MessageCreate) {
		// Check if the message is a system message for a new member joining
		if e.Message.Type == discord.MessageTypeUserJoin {
			var cookieEmoji discord.Emoji
			emoji, err := e.Client().Rest.GetEmoji(*e.GuildID, 1365671553622343792)
			if err != nil {
				log.Error("failed to fetch custom cookie~1 emoji", "err", err)
			} else {
				cookieEmoji = *emoji
			}
			found := cookieEmoji.ID != 0
			if found {
				if err := e.Client().Rest.AddReaction(e.ChannelID, e.Message.ID, cookieEmoji.String()); err != nil {
					log.Error("failed to add custom cookie~1 emoji reaction", "err", err)
				}
			} else {
				if err := e.Client().Rest.AddReaction(e.ChannelID, e.Message.ID, "🍪"); err != nil {
					log.Error("failed to add 🍪 emoji reaction", "err", err)
				}
			}
		}
	})
}

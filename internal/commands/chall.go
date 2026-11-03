package commands

import (
	"math/rand/v2"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctf"
)

var challCommand = discord.SlashCommandCreate{
	Name:        "chall",
	Description: "Create a thread for discuss about a challenge",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "name",
			Description: "The name of the challenge",
			Required:    true,
		},
		discord.ApplicationCommandOptionString{
			Name:        "description",
			Description: "The description of the challenge",
		},
		discord.ApplicationCommandOptionString{
			Name:        "category",
			Description: "The category of the challenge",
		},
	},
}

func (h *Handlers) chall(e *handler.CommandEvent) error {
	c, ok, err := h.channelCTF(e, e.Channel().ID())
	if !ok {
		return err
	}
	if !participates(e, c) {
		return reply(e, "You must be participating in this CTF to create a challenge thread. ❌")
	}
	active, err := ctf.IsActive(e.Client(), c)
	if err != nil {
		return err
	}
	if !active {
		return reply(e, "This CTF is not currently active. ❌")
	}

	options := e.SlashCommandInteractionData()
	name := options.String("name")
	embed := discord.Embed{
		Title:       "Challenge: " + name,
		Description: "A new challenge discussion thread has been created!",
		Color:       rand.IntN(0xFFFFFF), //nolint:gosec // Not security sensitive.
	}
	if category, _ := options.OptString("category"); category != "" {
		embed.Fields = append(embed.Fields, discord.EmbedField{Name: "Category", Value: category})
	}
	if description, _ := options.OptString("description"); description != "" {
		embed.Fields = append(embed.Fields, discord.EmbedField{Name: "Description", Value: description})
	}

	msg, err := e.Client().Rest.CreateMessage(e.Channel().ID(), discord.MessageCreate{Embeds: []discord.Embed{embed}})
	if err != nil {
		return err
	}
	if _, err := e.Client().Rest.CreateThreadFromMessage(e.Channel().ID(), msg.ID, discord.ThreadCreateFromMessage{
		Name:                name,
		AutoArchiveDuration: discord.AutoArchiveDuration3d,
	}); err != nil {
		return err
	}
	return reply(e, "Thread created successfully! ✅")
}

package commands

import (
	"database"
	"discordutils"
	"models"
	"slices"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"golang.org/x/exp/rand"
)

var chall = discord.SlashCommandCreate{
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
			Required:    false,
		},
		discord.ApplicationCommandOptionString{
			Name:        "category",
			Description: "The category of the challenge",
			Required:    false,
		},
	},
}

func ChallHandler() handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		db := database.GetInstance().Connection()
		options := e.SlashCommandInteractionData()
		name := options.String("name")
		description, _ := options.OptString("description")
		category, _ := options.OptString("category")

		if err := e.DeferCreateMessage(true); err != nil {
			log.Error("failed to defer create message", "error", err)
			return err
		}

		if e.GuildID() == nil {
			log.Warn("Create command used outside of a guild", "user_id", e.User().ID)
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "This command can only be used inside a guild. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		var ctf models.CTF
		err := ctf.GetByChannelID(
			db,
			e.Channel().ID(),
			*e.GuildID(),
		)
		if err != nil {
			log.Error("failed to fetch CTF by channel ID", "error", err)
			return err
		}
		if ctf == (models.CTF{}) {
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "No CTF is associated with this channel. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		if !slices.Contains(e.Member().RoleIDs, ctf.RoleID) {
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "You must be participating in this CTF to create a challenge thread. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		active, err := discordutils.IsCTFActive(e.Client(), ctf)
		if err != nil {
			log.Error("failed to check CTF status", "error", err)
			return err
		}
		if !active {
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "This CTF is not currently active. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		// Create the challenge thread
		embed := discord.Embed{
			Title:       "Challenge: " + name,
			Description: "A new challenge discussion thread has been created!",
			Color:       rand.Intn(0xFFFFFF),
			Fields:      []discord.EmbedField{},
		}
		if category != "" {
			embed.Fields = append(embed.Fields, discord.EmbedField{
				Name:  "Category",
				Value: category,
			})
		}
		if description != "" {
			embed.Fields = append(embed.Fields, discord.EmbedField{
				Name:  "Description",
				Value: description,
			})
		}
		createdMsg, err := e.Client().Rest.CreateMessage(e.Channel().ID(), discord.MessageCreate{
			Embeds: []discord.Embed{embed},
		})
		if err != nil {
			log.Error("failed to create message", "error", err)
			return err
		}

		if _, err := e.Client().Rest.CreateThreadFromMessage(e.Channel().ID(), createdMsg.ID, discord.ThreadCreateFromMessage{
			Name:                name,
			AutoArchiveDuration: discord.AutoArchiveDuration3d,
		}); err != nil {
			log.Error("failed to create thread", "error", err)
			_, _ = e.CreateFollowupMessage(discord.MessageCreate{
				Content: "failed to create the thread. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		_, err = e.CreateFollowupMessage(discord.MessageCreate{
			Content: "Thread created successfully! ✅",
			Flags:   discord.MessageFlagEphemeral,
		})
		if err != nil {
			log.Error("failed to send followup", "error", err)
			return err
		}

		return nil
	}
}

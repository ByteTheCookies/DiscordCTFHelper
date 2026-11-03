package commands

import (
	"database"
	"fmt"
	"models"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
)

var flag = discord.SlashCommandCreate{
	Name:        "flag",
	Description: "Register a flag in the ctf.",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "flag",
			Description: "The flag.",
			Required:    true,
		},
		discord.ApplicationCommandOptionUser{
			Name:        "mate",
			Description: "If you solved the challenge together with someone, specify their tag",
			Required:    false,
		},
		discord.ApplicationCommandOptionString{
			Name:        "challenge_name",
			Description: "the challenge name (optional)",
			Required:    false,
		},
	},
}

func FlagHandler() handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		db := database.GetInstance().Connection()
		client := e.Client()

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

		options := e.SlashCommandInteractionData()
		flag := options.String("flag")
		mate, okMate := options.OptMember("mate")
		challengeName, okChallenge := options.OptString("challenge_name")

		channelMessage := e.Channel().ID()

		activesThreads, err := e.Client().Rest.GetActiveGuildThreads(*e.GuildID())
		if err != nil {
			log.Error("failed to fetch active threads", "error", err)
			return err
		}

		var threadID snowflake.ID
		for _, thread := range activesThreads.Threads {
			if e.Channel().ID() == thread.ID() {
				threadID = thread.ID()
				channelMessage = *thread.ParentID()
				break
			}
		}

		var ctf models.CTF
		err = ctf.GetByChannelID(db, channelMessage, *e.GuildID())
		if err != nil {
			log.Error("failed to fetch ctf by channel id", "error", err)
			return err
		}
		if ctf == (models.CTF{}) {
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "No CTFs are currently active in channel. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		if threadID != 0 {
			locked := true
			if _, err := e.Client().Rest.UpdateChannel(threadID, discord.GuildThreadUpdate{Locked: &locked}); err != nil {
				log.Error("failed to lock challenge thread", "error", err)
			}
		}

		var report models.Report
		err = report.GetByCTFID(db, ctf.ID)
		if err != nil {
			log.Error("failed to fetch report for ctf", "error", err)
			return err
		}

		if report == (models.Report{}) {
			report = models.Report{
				CTFID:  ctf.ID,
				Place:  -1,
				Score:  -1,
				Solves: 1,
			}
			if err := report.Add(db); err != nil {
				log.Error("failed to save report", "error", err)
			}
		} else {
			report.Solves += 1
			report.CTFID = ctf.ID
			if err := report.Update(db); err != nil {
				log.Error("failed to update report", "error", err)
			}
		}

		content := fmt.Sprintf("<@&%s> NEW FLAG FOUND BY %s", ctf.RoleID, e.User().Mention())
		// The flag and challenge name are user text: only ping who this message is meant for.
		mentions := discord.AllowedMentions{
			Parse: []discord.AllowedMentionType{},
			Roles: []snowflake.ID{ctf.RoleID},
			Users: []snowflake.ID{e.User().ID},
		}
		if okMate {
			content += " and " + mate.User.Mention()
			mentions.Users = append(mentions.Users, mate.User.ID)
		}
		if okChallenge {
			content += " for challenge: " + challengeName
		}

		content += fmt.Sprintf("🎉\n> `%s`", flag)

		msg, err := client.Rest.CreateMessage(channelMessage, discord.MessageCreate{
			Content:         content,
			AllowedMentions: &mentions,
		})
		if err != nil {
			log.Error("failed to send flag message", "error", err)
			return err
		}

		err = client.Rest.AddReaction(channelMessage, msg.ID, "🔥")
		if err != nil {
			log.Error("failed to add reaction to flag message", "error", err)
			return err
		}

		_, err = e.CreateFollowupMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Flag registered successfully! ✅\nTotal solves for this CTF: %d", report.Solves),
			Flags:   discord.MessageFlagEphemeral,
		})
		return err
	}
}

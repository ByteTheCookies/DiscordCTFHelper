package commands

import (
	"ctftime"
	"database"
	"discordutils"
	"errors"
	"fmt"
	"models"
	"time"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
)

var create = discord.SlashCommandCreate{
	Name:        "create",
	Description: "Create a CTF event in the discord server.",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionInt{
			Name:        "ctftime_id",
			Description: "The ID of the ctf on ctftime",
			Required:    true,
		},
	},
}

func CreateCTF(guildId *snowflake.ID, client *bot.Client, ctfTimeId int, server models.Server) error {
	db := database.GetInstance().Connection()
	data, err := ctftime.GetCTFInfo(ctfTimeId)
	if err != nil {
		log.Error("failed to retrieve CTF information", "error", err)
		return errors.New("failed to get the information of the CTF. ❌")
	}

	log.Debug("Retrieved CTF information", "data", data)
	if data.Start == "" {
		log.Error("Invalid start time from CTF information")
		return errors.New("failed to get the information of the CTF. ❌")
	}

	if data.Finish == "" {
		log.Error("Invalid finish time from CTF information")
		return errors.New("failed to get the information of the CTF. ❌")
	}

	startTime, err := time.Parse(time.RFC3339, data.Start)
	if err != nil {
		log.Error("failed to parse CTF start time", "error", err)
		return errors.New("failed to get the information of the CTF. ❌")
	}

	endTime, err := time.Parse(time.RFC3339, data.Finish)
	if err != nil {
		log.Error("failed to parse CTF end time", "error", err)
		return errors.New("failed to get the information of the CTF. ❌")
	}

	title := fmt.Sprintf("%s - %d", data.Title, startTime.Year())

	ctf := models.CTF{
		Name:      title,
		ServerID:  *guildId,
		CTFTimeID: int64(ctfTimeId),
	}
	present, err := ctf.IsPresent(db)
	if err != nil {
		log.Error("failed to check existing CTF", "error", err)
		return err
	}
	if present {
		return errors.New("the CTF is already present in the discord server. ❌")
	}

	// Save the CTF record early so progress is not lost
	if err := ctf.Add(db); err != nil {
		log.Error("failed to add CTF to database", "error", err)
		return err
	}
	log.Info("CTF record created in database", "title", title)

	log.Info("Creating role and channel for CTF", "title", title)
	role, err := discordutils.CreateRole(
		client,
		guildId,
		title,
	)
	if err != nil {
		log.Error("failed to create role", "error", err)
		return err
	}
	if role == nil {
		return errors.New("failed to create role")
	}

	log.Debug("Created role for CTF", "role_id", role.ID)
	ctf.RoleID = role.ID
	if err := ctf.Update(db); err != nil {
		log.Error("failed to update CTF with role ID", "error", err)
		return err
	}

	log.Debug("Creating channel for CTF", "title", title)
	channelCTF, err := discordutils.CreateChannel(
		client,
		guildId,
		title,
		server.ActiveCategoryID,
		role.ID,
		server.RoleManagerID,
	)
	if err != nil {
		log.Error("failed to create channel", "error", err)
		return err
	}
	if channelCTF == nil {
		return errors.New("failed to create channel")
	}

	log.Debug("Created channel for CTF", "channel_id", (*channelCTF).ID())
	ctf.TextChannelID = (*channelCTF).ID()
	if err := ctf.Update(db); err != nil {
		log.Error("failed to update CTF with channel ID", "error", err)
		return err
	}

	log.Info("Sending welcome message and pinning link", "channel_id", (*channelCTF).ID())
	welcomeContent := fmt.Sprintf("%s Welcome to the CTF **%s**! 🎉", role.Mention(), title)
	if _, err := client.Rest.CreateMessage((*channelCTF).ID(), discord.MessageCreate{
		Content: welcomeContent,
	}); err != nil {
		log.Error("failed to send welcome message", "error", err)
		return err
	}

	log.Debug("Sending link message and pinning", "channel_id", (*channelCTF).ID())
	linkMessage, err := client.Rest.CreateMessage((*channelCTF).ID(), discord.MessageCreate{
		Content: "Link to ctf: " + data.URL,
	})
	if err != nil {
		log.Error("failed to send link message", "error", err)
		return err
	}

	if err := client.Rest.PinMessage((*channelCTF).ID(), linkMessage.ID); err != nil {
		log.Error("failed to pin message", "error", err)
	}

	log.Info("Creating embed and scheduled event for CTF", "title", title)
	feedChannel, err := client.Rest.GetChannel(server.FeedChannelID)
	if err != nil {
		log.Error("failed to fetch feed channel", "error", err)
		return err
	}

	log.Debug("Fetched feed channel", "channel_id", feedChannel.ID())

	log.Debug("Creating embed message for CTF", "title", title)
	embedMsg, err := discordutils.CreateEmbed(
		client,
		data,
		startTime,
		endTime,
		feedChannel,
	)
	if err != nil {
		log.Error("failed to create embed", "error", err)
		return err
	}
	if embedMsg == nil {
		return errors.New("failed to create embed message")
	}

	ctf.MsgID = embedMsg.ID
	if err := ctf.Update(db); err != nil {
		log.Error("failed to update CTF with embed message ID", "error", err)
		return err
	}

	description := discordutils.Truncate(data.Description, discordutils.MaxEventDescLength)

	ctf.Description = description
	if err := ctf.Update(db); err != nil {
		log.Error("failed to update CTF with description", "error", err)
		return err
	}

	log.Info("Creating scheduled event for CTF", "title", title)
	// Ensure start time is after current time to avoid Discord API errors
	fixStartTime := startTime
	if fixStartTime.Before(time.Now().Add(time.Minute)) {
		fixStartTime = time.Now().Add(time.Minute * 1)
	}

	events, err := discordutils.CreateEvents(
		client,
		guildId,
		data,
		description,
		fixStartTime,
		endTime,
	)
	if err != nil {
		log.Error("failed to create events", "error", err)
		return err
	}
	if events == (discord.GuildScheduledEvent{}) {
		return errors.New("failed to create scheduled event")
	}

	ctf.EventID = events.ID
	if err := ctf.Update(db); err != nil {
		log.Error("failed to update CTF with event ID", "error", err)
		return err
	}

	log.Info("CTF fully created and saved", "title", title)
	return nil
}

func CreateHandler() handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		db := database.GetInstance().Connection()
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
		ctfTimeID := options.Int("ctftime_id")

		var server models.Server
		err := server.GetByID(db, *e.GuildID())
		if err != nil {
			log.Error("failed to fetch server configuration", "error", err)
			_, _ = e.CreateFollowupMessage(discord.MessageCreate{
				Content: "failed to fetch server configuration. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		if server == (models.Server{}) {
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "The server is not configured. ❌ Please run the /init command.",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		if server.ActiveCategoryID == 0 || server.RoleManagerID == 0 {
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "failed to set the category. ❌ Please check the configuration or contact support.",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		allowed, err := discordutils.CheckPermission(e)
		if err != nil || !allowed {
			return err
		}

		err = CreateCTF(e.GuildID(), e.Client(), ctfTimeID, server)
		if err != nil {
			_, _ = e.CreateFollowupMessage(discord.MessageCreate{
				Content: err.Error(),
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		// Fetch the created CTF to confirm and announce
		var ctf models.CTF
		err = ctf.GetByTimeID(db, int64(ctfTimeID))
		if err != nil {
			log.Error("failed to fetch CTF after creation", "error", err)
			_, _ = e.CreateFollowupMessage(discord.MessageCreate{
				Content: "failed to fetch CTF after creation. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}
		if ctf == (models.CTF{}) {
			log.Error("CTF not found after creation")
			_, _ = e.CreateFollowupMessage(discord.MessageCreate{
				Content: "CTF not found after creation. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return nil
		}

		feedChannel, err := e.Client().Rest.GetChannel(server.FeedChannelID)
		if err != nil {
			log.Error("failed to fetch feed channel", "error", err)
			_, _ = e.CreateFollowupMessage(discord.MessageCreate{
				Content: "failed to fetch feed channel. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		if teamRole, _ := e.Client().Rest.GetRole(*e.GuildID(), server.RoleTeamID); teamRole != nil {
			interactionChannel := e.ApplicationCommandInteraction.Channel()
			feedMention := fmt.Sprintf("<#%s>", feedChannel.ID().String())
			if _, err := e.Client().Rest.CreateMessage(interactionChannel.ID(), discord.MessageCreate{
				Content: fmt.Sprintf("%s New CTF published in %s 🎉", teamRole.Mention(), feedMention),
			}); err != nil {
				log.Error("failed to announce CTF to team role", "error", err)
			}
		}

		_, _ = e.CreateFollowupMessage(discord.MessageCreate{
			Content: "CTF created in the discord server ✅",
			Flags:   discord.MessageFlagEphemeral,
		})
		return nil
	}
}

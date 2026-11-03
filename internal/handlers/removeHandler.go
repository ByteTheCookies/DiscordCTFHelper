package handlers

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/database"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/discordutils"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/models"
)

func RemoveCTF(e *events.ComponentInteractionCreate, ctf models.CTF) error {
	db := database.GetInstance().Connection()

	// A channel already gone, or never created by a failed /create, must not make the CTF unremovable.
	if ctf.TextChannelID != 0 {
		err := e.Client().Rest.DeleteChannel(ctf.TextChannelID)
		var restErr *rest.Error
		if err != nil && (!errors.As(err, &restErr) || restErr.Code != rest.JSONErrorCodeUnknownChannel) {
			log.Error("Error deleting channel for CTF", "err", err, "channel_id", ctf.TextChannelID)
			return err
		}
		log.Info("Deleted channel for CTF", "channel_id", ctf.TextChannelID, "ctf_time_id", ctf.CTFTimeID)
	}

	if ctf.RoleID != 0 {
		if err := e.Client().Rest.DeleteRole(ctf.ServerID, ctf.RoleID); err != nil {
			log.Error("Error deleting role for CTF", "err", err, "role_id", ctf.RoleID)
		} else {
			log.Info("Deleted role for CTF", "role_id", ctf.RoleID, "ctf_time_id", ctf.CTFTimeID)
		}
	}

	if ctf.EventID != 0 {
		if err := e.Client().Rest.DeleteGuildScheduledEvent(ctf.ServerID, ctf.EventID); err != nil {
			log.Error("Error deleting event for CTF", "err", err, "event_id", ctf.EventID)
		} else {
			log.Info("Deleted event for CTF", "event_id", ctf.EventID, "ctf_time_id", ctf.CTFTimeID)
		}
	}

	var server models.Server
	if err := server.GetByID(db, ctf.ServerID); err != nil {
		log.Error("Error fetching server for CTF removal", "err", err, "server_id", ctf.ServerID)
		return err
	}

	if ctf.MsgID != 0 {
		if err := e.Client().Rest.DeleteMessage(server.FeedChannelID, ctf.MsgID); err != nil {
			log.Error("Error deleting feed message for CTF", "err", err, "msg_id", ctf.MsgID)
		} else {
			log.Info("Deleted feed message for CTF", "msg_id", ctf.MsgID, "ctf_time_id", ctf.CTFTimeID)
		}
	}

	return nil
}

func RemoveHandler() bot.EventListener {
	return bot.NewListenerFunc(func(e *events.ComponentInteractionCreate) {
		if e.Data.CustomID() != "remove_ctf_select" || e.GuildID() == nil {
			return
		}
		values := e.StringSelectMenuInteractionData().Values
		log.Debug("Select menu interaction", "values", values, "user_id", e.User().ID)

		allowed, err := discordutils.IsManager(*e.GuildID(), e.Member())
		if err != nil {
			log.Error("Error checking permissions", "err", err, "user_id", e.User().ID)
		}
		if !allowed {
			_ = e.CreateMessage(discord.MessageCreate{
				Content: discordutils.PermissionDenied,
				Flags:   discord.MessageFlagEphemeral,
			})
			return
		}

		// Removal takes several Discord calls: acknowledge within the 3 second limit.
		if err := e.DeferCreateMessage(true); err != nil {
			log.Error("failed to defer create message", "err", err)
			return
		}
		reply := func(content string) {
			if _, err := e.Client().Rest.CreateFollowupMessage(e.ApplicationID(), e.Token(), discord.MessageCreate{
				Content: content,
				Flags:   discord.MessageFlagEphemeral,
			}); err != nil {
				log.Error("failed to send followup", "err", err)
			}
		}

		db := database.GetInstance().Connection()
		for _, value := range values {
			valueInt, err := strconv.Atoi(value)
			if err != nil {
				log.Error("Error parsing CTF ID from select value", "err", err, "value", value)
				reply("Error processing selection.")
				return
			}

			var ctf models.CTF
			err = ctf.GetByID(db, int64(valueInt))
			if err != nil {
				log.Error("Error fetching CTF by ID", "err", err, "ctf_id", valueInt)
				reply("Error retrieving the CTF from the database.")
				return
			}
			if ctf == (models.CTF{}) || ctf.ServerID != *e.GuildID() {
				reply(fmt.Sprintf("No CTF found with ID %d.", valueInt))
				return
			}

			if err := RemoveCTF(e, ctf); err != nil {
				log.Error("Error removing CTF", "err", err, "ctf_id", valueInt)
				reply("Error while removing the CTF.")
				return
			}

			// Delete CTF from database
			if err := ctf.Delete(db); err != nil {
				log.Error("Error deleting CTF from database", "err", err, "ctf_id", ctf.ID)
				reply("Error while removing the CTF.")
				return
			}
		}

		reply("CTF removed successfully ✅.")
	})
}

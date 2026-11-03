package handlers

import (
	"database"
	"discordutils"
	"fmt"
	"models"
	"strconv"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func RemoveCTF(e *events.ComponentInteractionCreate, ctf models.CTF) error {
	db := database.GetInstance().Connection()
	channel, err := e.Client().Rest.GetChannel(ctf.TextChannelID)
	if err != nil {
		log.Error("Error fetching channel to remove CTF", "err", err, "channel_id", ctf.TextChannelID)
		return err
	}

	if err := e.Client().Rest.DeleteChannel(ctf.TextChannelID); err != nil {
		log.Error("Error deleting channel for CTF", "err", err, "channel_id", ctf.TextChannelID)
		return err
	}
	log.Info("Deleted channel for CTF", "channel_id", channel.ID(), "ctf_time_id", ctf.CTFTimeID)

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
	err = server.GetByID(db, ctf.ServerID)
	if err != nil {
		log.Error("Error fetching server for CTF removal", "err", err, "server_id", ctf.ServerID)
		return err
	}
	feedChannel, err := e.Client().Rest.GetChannel(server.FeedChannelID)
	if err != nil {
		log.Error("Error fetching feed channel for CTF removal", "err", err, "channel_id", server.FeedChannelID)
		return err
	}

	if ctf.MsgID != 0 {
		if err := e.Client().Rest.DeleteMessage(feedChannel.ID(), ctf.MsgID); err != nil {
			log.Error("Error deleting feed message for CTF", "err", err, "msg_id", ctf.MsgID)
		} else {
			log.Info("Deleted feed message for CTF", "msg_id", ctf.MsgID, "ctf_time_id", ctf.CTFTimeID)
		}
	}

	return nil
}

func RemoveHandler() bot.EventListener {
	return bot.NewListenerFunc(func(e *events.ComponentInteractionCreate) {
		db := database.GetInstance().Connection()

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

		for _, value := range values {
			valueInt, err := strconv.Atoi(value)
			if err != nil {
				log.Error("Error parsing CTFTime ID from select value", "err", err, "value", value)
				_ = e.CreateMessage(discord.MessageCreate{
					Content: "Error processing selection.",
					Flags:   discord.MessageFlagEphemeral,
				})
				return
			}

			var ctf models.CTF
			err = ctf.GetByID(db, int64(valueInt))
			if err != nil {
				log.Error("Error fetching CTF by CTFTime ID", "err", err, "ctf_time_id", valueInt)
				_ = e.CreateMessage(discord.MessageCreate{
					Content: "Error retrieving the CTF from the database.",
					Flags:   discord.MessageFlagEphemeral,
				})
				return
			}
			if ctf == (models.CTF{}) || ctf.ServerID != *e.GuildID() {
				_ = e.CreateMessage(discord.MessageCreate{
					Content: fmt.Sprintf("No CTF found with CTFTime ID %d.", valueInt),
					Flags:   discord.MessageFlagEphemeral,
				})
				return
			}

			if err := RemoveCTF(e, ctf); err != nil {
				log.Error("Error removing CTF", "err", err, "ctf_time_id", valueInt)
				_ = e.CreateMessage(discord.MessageCreate{
					Content: "Error while removing the CTF.",
					Flags:   discord.MessageFlagEphemeral,
				})
				return
			}

			// Delete CTF from database
			if err := ctf.Delete(db); err != nil {
				log.Error("Error deleting CTF from database", "err", err, "ctf_id", ctf.ID)
				_ = e.CreateMessage(discord.MessageCreate{
					Content: "Error while removing the CTF.",
					Flags:   discord.MessageFlagEphemeral,
				})
				return
			}
		}

		_ = e.CreateMessage(discord.MessageCreate{
			Content: "CTF removed successfully ✅.",
			Flags:   discord.MessageFlagEphemeral,
		})
	})
}

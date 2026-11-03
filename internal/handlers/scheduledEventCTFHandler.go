package handlers

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/database"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/models"
)

func ScheduleEventUpdateHandler() bot.EventListener {
	return bot.NewListenerFunc(
		func(e *events.GuildScheduledEventUpdate) {
			db := database.GetInstance().Connection()

			var ctf models.CTF
			err := ctf.GetByEventID(db, e.GuildScheduled.ID, e.GuildScheduled.GuildID)
			if err != nil {
				log.Error("Error fetching CTF for scheduled event update", "err", err, "event_id", e.GuildScheduled.ID, "guild_id", e.GuildScheduled.GuildID)
				return
			}
			if ctf == (models.CTF{}) {
				log.Info("CTF not found in database for scheduled event update", "ctf_name", e.GuildScheduled.Name)
				return
			}

			// If status changed to active, announce start
			if e.OldGuildScheduled.Status != e.GuildScheduled.Status && e.GuildScheduled.Status == discord.ScheduledEventStatusActive {
				channel, err := e.Client().Rest.GetChannel(ctf.TextChannelID)
				if err == nil && channel.Type() == discord.ChannelTypeGuildText {
					_, err := e.Client().Rest.CreateMessage(channel.ID(), discord.MessageCreate{
						Content: fmt.Sprintf("<@&%d> The CTF has started! Good luck to all participants! :tada:", ctf.RoleID),
					})
					if err != nil {
						log.Error("failed to send CTF started message", "err", err, "channel_id", channel.ID())
					}
				}
			}

			// If status changed to completed, archive channel and update role
			if e.OldGuildScheduled.Status != e.GuildScheduled.Status && e.GuildScheduled.Status == discord.ScheduledEventStatusCompleted {
				channel, err := e.Client().Rest.GetChannel(ctf.TextChannelID)
				if err != nil {
					log.Error("failed to fetch CTF text channel for archiving", "err", err, "channel_id", ctf.TextChannelID)
					return
				}

				var server models.Server
				err = server.GetByID(db, ctf.ServerID)
				if err != nil {
					log.Error("failed to fetch server for CTF archiving", "err", err, "server_id", ctf.ServerID)
					return
				}

				// Move channel to archive category if possible
				pos := 0
				if server != (models.Server{}) && server.ArchiveCategoryID != 0 && channel.Type() == discord.ChannelTypeGuildText {
					_, err := e.Client().Rest.UpdateChannel(channel.ID(), discord.GuildTextChannelUpdate{
						ParentID: &server.ArchiveCategoryID,
						Position: &pos,
					})
					if err != nil {
						log.Error("failed to move channel to archive category", "err", err, "channel_id", channel.ID(), "archive_category_id", server.ArchiveCategoryID)
					}
				}

				// Update role color, hoist, mentionable
				if ctf.RoleID != 0 {
					_, err := e.Client().Rest.UpdateRole(ctf.ServerID, ctf.RoleID, discord.RoleUpdate{
						Color:       func(v int) *int { return &v }(0xD3D3D3),
						Hoist:       func(v bool) *bool { return &v }(false),
						Mentionable: func(v bool) *bool { return &v }(false),
					})
					if err != nil {
						log.Error("failed to update CTF role after event completion", "err", err, "role_id", ctf.RoleID)
					}
				}

				// Send archive message
				if channel.Type() == discord.ChannelTypeGuildText {
					_, err := e.Client().Rest.CreateMessage(channel.ID(), discord.MessageCreate{
						Content: fmt.Sprintf("<@&%d> The CTF **%s** has ended! The channel has been moved to the archived category.", ctf.RoleID, ctf.Name),
					})
					if err != nil {
						log.Error("failed to send CTF archived message", "err", err, "channel_id", channel.ID())
					}
				}
			}
		})
}

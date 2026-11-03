package handlers

import (
	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/database"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/models"
)

func ReactionAddFeedMessageHandler() bot.EventListener {
	return bot.NewListenerFunc(func(e *events.MessageReactionAdd) {
		db := database.GetInstance().Connection()

		// Only handle guild reactions
		log.Debug("Reaction add event received", "message_id", e.MessageID, "user_id", e.UserID)
		if e.GuildID == nil {
			return
		}

		// Fetch the guild member who added the reaction
		member := e.Member
		if member == nil || member.User.Bot {
			return
		}

		log.Debug("Reaction add event", "message_id", e.MessageID, "user_id", member.User.ID)

		// Find CTF by message ID and guild ID
		var ctf models.CTF
		err := ctf.GetByMessageID(db, e.MessageID, *e.GuildID)
		if err != nil {
			log.Error("Error fetching CTF by message ID", "err", err, "message_id", e.MessageID)
			return
		}
		if ctf == (models.CTF{}) {
			log.Info("CTF not found for message", "message_id", e.MessageID)
			return
		}

		// Add the CTF role to the member
		if ctf.RoleID != 0 {
			if err := e.Client().Rest.AddMemberRole(*e.GuildID, member.User.ID, ctf.RoleID); err != nil {
				log.Error("failed to add CTF role to member", "err", err, "role_id", ctf.RoleID, "user_id", member.User.ID)
			}
		}
	})
}

func ReactionRemoveFeedMessageHandler() bot.EventListener {
	return bot.NewListenerFunc(func(e *events.MessageReactionRemove) {
		db := database.GetInstance().Connection()

		log.Debug("Reaction remove event received", "message_id", e.MessageID, "user_id", e.UserID)
		// Only handle guild reactions
		if e.GuildID == nil {
			return
		}

		member, err := e.Client().Rest.GetMember(*e.GuildID, e.UserID)
		if err != nil {
			log.Error("failed to fetch member for reaction remove", "err", err, "user_id", e.UserID)
			return
		}
		if member.User.Bot {
			return
		}

		log.Debug("Reaction remove event", "message_id", e.MessageID, "user_id", member.User.ID)

		// Find CTF by message ID and guild ID
		var ctf models.CTF
		err = ctf.GetByMessageID(db, e.MessageID, *e.GuildID)
		if err != nil {
			log.Error("Error fetching CTF by message ID", "err", err, "message_id", e.MessageID)
			return
		}
		if ctf == (models.CTF{}) {
			log.Info("CTF not found for message", "message_id", e.MessageID)
			return
		}

		// Remove the CTF role from the member
		if ctf.RoleID != 0 {
			if err := e.Client().Rest.RemoveMemberRole(*e.GuildID, member.User.ID, ctf.RoleID); err != nil {
				log.Error("failed to remove CTF role from member", "err", err, "role_id", ctf.RoleID, "user_id", member.User.ID)
			}
		} else {
			log.Info("Role not found for CTF", "ctf_name", ctf.Name)
		}
	})
}

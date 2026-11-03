package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/database"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/models"
)

func CredsModalListener() bot.EventListener {
	// Handle all the modal submission
	return bot.NewListenerFunc(func(e *events.ModalSubmitInteractionCreate) {
		db := database.GetInstance().Connection()
		modalID := e.Data.CustomID
		log.Debug("Modal submitted", "modal_id", modalID, "user_id", e.User().ID)
		var content, username, password string
		personal := false
		switch {
		case strings.HasPrefix(modalID, "creds_modal_"):
			// Gather all modal input values for logging or debugging
			for component := range e.Data.AllComponents() {
				if input, ok := component.(discord.TextInputComponent); ok {
					switch input.CustomID {
					case "username":
						username = input.Value
					case "password":
						password = input.Value
					}
					if input.CustomID == "personal" {
						personalInput := strings.ToLower(input.Value)
						if personalInput == "yes" {
							personal = true
						} else {
							personal = false
						}
					}
				}
			}

			ctfIDStr := strings.TrimPrefix(modalID, "creds_modal_")
			ctfID, err := strconv.Atoi(ctfIDStr)
			if err != nil {
				log.Error("Error parsing CTF ID from modal ID", "err", err, "modal_id", modalID)
				content = "Error processing modal submission."
				break
			}

			var ctf models.CTF
			err = ctf.GetByChannelID(db, e.Channel().ID(), *e.GuildID())
			if err != nil {
				log.Error("Error fetching ctf", "err", err, "guild_id", *e.GuildID())
				content = "Error processing modal submission."
				break
			}
			if ctf.ID != int64(ctfID) {
				log.Error("CTF ID mismatch", "expected_ctf_id", ctf.ID, "modal_ctf_id", ctfID)
				content = "Error processing modal submission."
				break
			}

			// Save the credentials to the database
			creds := models.Creds{
				Username: username,
				Password: password,
				Personal: personal,
				CTFID:    int64(ctfID),
			}
			err = creds.Add(db)
			if err != nil {
				log.Error("Error adding credentials to database", "err", err, "ctf_id", ctfID)
				content = "Error saving credentials."
				break
			}
			content += fmt.Sprintf("<@&%s>, Credentials submitted ✅.", ctf.RoleID)
		default:
			content = "Unknown modal submitted."
		}

		if err := e.CreateMessage(discord.MessageCreate{
			Content: content,
		}); err != nil {
			log.Error("error creating modal", "err", err)
		}
	})
}

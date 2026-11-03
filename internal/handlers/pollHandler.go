package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/commands"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/database"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/models"
)

func VotePollHandler() bot.EventListener {
	return bot.NewListenerFunc(func(e *events.MessageCreate) {
		// This runs for every message: leave before touching the database.
		if e.GuildID == nil || e.Message.Type != discord.MessageTypePollResult || e.Message.Author.ID != e.Client().ID() {
			return
		}

		db := database.GetInstance().Connection()
		var server models.Server
		err := server.GetByID(db, *e.GuildID)
		if err != nil {
			log.Error("Error fetching server configuration", "err", err, "guild_id", *e.GuildID)
			return
		}

		if e.Message.MessageReference == nil || e.Message.MessageReference.MessageID == nil {
			log.Error("Poll result message does not have a message reference",
				"guild_id", *e.GuildID,
				"channel_id", e.ChannelID,
				"message_id", e.Message.ID,
			)
			return
		}

		message, err := e.Client().Rest.GetMessage(e.ChannelID, *e.Message.MessageReference.MessageID)
		if err != nil {
			log.Error("Failed to fetch original message for poll result",
				"err", err,
				"channel_id", e.ChannelID,
				"message_id", e.Message.MessageReference.MessageID,
			)
			return
		}

		if message.Poll == nil || message.Poll.Results == nil {
			log.Error("Original message does not contain poll data",
				"channel_id", e.ChannelID,
				"original_message_id", message.ID,
			)
			return
		}

		maxVotes := -1
		for _, ans := range message.Poll.Results.AnswerCounts {
			if ans.Count > maxVotes {
				maxVotes = ans.Count
			}
		}

		if maxVotes <= 0 {
			log.Warn("No votes in the poll",
				"guild_id", *e.GuildID,
				"channel_id", e.ChannelID,
				"message_id", message.ID,
			)
			return
		}

		var winningIndices []int
		for _, ans := range message.Poll.Results.AnswerCounts {
			if ans.Count == maxVotes {
				winningIndices = append(winningIndices, ans.ID-1)
			}
		}

		if len(winningIndices) == 1 {
			maxIndex := winningIndices[0]
			if maxIndex >= 0 && maxIndex < len(message.Poll.Answers) {
				bestAnswer := *message.Poll.Answers[maxIndex].PollMedia.Text
				log.Info("Most voted answer found", "answer", bestAnswer, "votes", maxVotes)

				openParen := strings.LastIndex(bestAnswer, "(")
				closeParen := strings.LastIndex(bestAnswer, ")")

				if openParen == -1 || closeParen == -1 || openParen >= closeParen {
					log.Error("Failed to parse CTFTime ID from answer text",
						"answer", bestAnswer,
					)
					return
				}

				ctftimeID := strings.TrimSpace(bestAnswer[openParen+1 : closeParen])
				ctfTimeIDInt, err := strconv.Atoi(ctftimeID)
				if err != nil {
					log.Error("Error during CTFTime ID conversion", "err", err, "ctf_time_id", ctftimeID)
					return
				}

				if err := commands.CreateCTF(e.GuildID, e.Client(), ctfTimeIDInt, server); err != nil {
					log.Error("Failed to create the CTF that won the poll", "err", err, "ctf_time_id", ctfTimeIDInt)
					if _, err := e.Client().Rest.CreateMessage(e.ChannelID, discord.MessageCreate{
						Content: fmt.Sprintf("<@&%s>, failed to create the CTF that won the poll. ❌", server.RoleManagerID),
					}); err != nil {
						log.Error("Failed to report poll failure", "err", err)
					}
				}
			} else {
				log.Warn("Invalid maxIndex in poll answers",
					"max_index", maxIndex,
					"answers_len", len(message.Poll.Answers),
				)
			}
		} else {
			_, err := e.Client().Rest.CreateMessage(e.ChannelID, discord.MessageCreate{
				Content: fmt.Sprintf("<@&%s>, The last poll resulted in a tie.", server.RoleTeamID),
			})
			if err != nil {
				log.Error("Failed to send tie message", "err", err)
			}
		}
	})
}

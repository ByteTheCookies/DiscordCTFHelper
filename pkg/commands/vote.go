package commands

import (
	"ctftime"
	"database"
	"fmt"
	"models"
	"time"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var vote = discord.SlashCommandCreate{
	Name:        "vote",
	Description: "Vote the next ctf to participate in",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionInt{
			Name:        "duration",
			Description: "Duration of the vote in hours (default 48h)",
			Required:    false,
		},
		discord.ApplicationCommandOptionBool{
			Name:        "onsite",
			Description: "Set to true to include also onsite CTFs (default false)",
			Required:    false,
		},
	},
}

func VoteHandler() handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		options := e.SlashCommandInteractionData()
		duration, ok := options.OptInt("duration")

		onsite, _ := options.OptBool("onsite")

		if e.GuildID() == nil {
			log.Warn("Create command used outside of a guild", "user_id", e.User().ID)
			if err := e.DeferCreateMessage(true); err != nil {
				log.Error("failed to defer create message", "error", err)
				return err
			}
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "This command can only be used inside a guild. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}


		log.Info("Fetching CTFs from ctftime", "onsite", onsite)
		ctfs, err := ctftime.GetCTFs(10, onsite)
		if err != nil {
			log.Error("failed to get CTFs", "error", err)
			return err
		}

		var ctfsThisWeek []ctftime.Event

		now := time.Now()

		// If it's Saturday or Sunday, look at next week's CTFs instead of this week.
		// This keeps the selection "flexible" near the weekend.
		weekday := now.Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			now = now.AddDate(0, 0, 7)
		}

		year, week := now.ISOWeek()
		firstStart := time.Now().Add(time.Duration(duration) * time.Hour)
		for _, ctf := range ctfs {
			startTime, err := time.Parse(time.RFC3339, ctf.Start)
			if err != nil {
				log.Warn("Unable to parse CTF start date", "ctf", ctf.Title, "start", ctf.Start, "error", err)
				continue
			}
			y, w := startTime.ISOWeek()
			if y == year && w == week {
				ctfsThisWeek = append(ctfsThisWeek, ctf)

				t1, _ := time.Parse(time.RFC3339, ctf.Start)
				if t1.Before(firstStart) {
					firstStart = t1
				}
			}
		}

		hoursToRemove := int((time.Now().Add(time.Duration(duration) * time.Hour).Sub(firstStart)).Hours())
		duration = duration - hoursToRemove

		log.Info("CTFs found for this week", "count", len(ctfsThisWeek))

		if len(ctfsThisWeek) < 2 || len(ctfsThisWeek) > 10 {
			if err := e.DeferCreateMessage(true); err != nil {
				log.Error("failed to defer create message", "error", err)
				return err
			}
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "There must be between 2 and 10 CTFs this week to create a vote. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		var server models.Server
		db := database.GetInstance().Connection()
		err = server.GetByID(db, *e.GuildID())
		if err != nil {
			log.Error("failed to fetch server configuration", "error", err)
			if err := e.DeferCreateMessage(true); err != nil {
				log.Error("failed to defer create message", "error", err)
				return err
			}
			_, _ = e.CreateFollowupMessage(discord.MessageCreate{
				Content: "failed to fetch server configuration. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		pollTitle := "Vote the next CTF to participate in! 🎉"
		log.Info("Creating vote poll", "ctfs_count", len(ctfsThisWeek))

		if !ok || duration <= 0 {
			duration = int(time.Until(firstStart).Hours()) - 12
			if duration < 1 {
				duration = max(1, int(time.Until(firstStart).Hours())/2)
			}
		}

		err = e.CreateMessage(
			discord.MessageCreate{
				Content: "It's time to vote for the next CTF! Cast your vote below: 🗳️",
				Poll: &discord.PollCreate{
					LayoutType:       discord.PollLayoutTypeDefault,
					AllowMultiselect: false,
					Question: discord.PollMedia{
						Text: &pollTitle,
					},
					Answers: func() []discord.PollMedia {
						options := make([]discord.PollMedia, 0, len(ctfsThisWeek))

						for _, ctf := range ctfsThisWeek {
							log.Debug("Adding CTF to poll options", "ctf", ctf.Title)
							text := ctf.Title + fmt.Sprintf(" (%d)", ctf.ID)
							options = append(options, discord.PollMedia{
								Text: &text,
							})
						}
						return options
					}(),
					Duration: duration,
				},
			},
		)

		return err
	}
}

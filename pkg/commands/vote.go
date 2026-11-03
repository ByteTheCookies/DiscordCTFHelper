package commands

import (
	"ctftime"
	"discordutils"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

// maxPollHours is the longest poll Discord accepts.
const maxPollHours = 768

var vote = discord.SlashCommandCreate{
	Name:        "vote",
	Description: "Vote the next ctf to participate in",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionInt{
			Name:        "duration",
			Description: "Duration of the vote in hours (default 48h)",
			Required:    false,
			MinValue:    new(1),
			MaxValue:    new(maxPollHours),
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
		duration, durationProvided := options.OptInt("duration")

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

		allowed, err := discordutils.IsManager(*e.GuildID(), e.Member())
		if err != nil {
			log.Error("failed to check permissions", "error", err)
			return err
		}
		if !allowed {
			return e.CreateMessage(discord.MessageCreate{
				Content: discordutils.PermissionDenied,
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		// CTFtime can take longer than the 3 seconds Discord allows before an ack.
		if err := e.DeferCreateMessage(true); err != nil {
			log.Error("failed to defer create message", "error", err)
			return err
		}
		fail := func(content string) error {
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: content + " ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		if durationProvided && (duration < 1 || duration > maxPollHours) {
			return fail(fmt.Sprintf("The duration must be between 1 and %d hours.", maxPollHours))
		}

		log.Info("Fetching CTFs from ctftime", "onsite", onsite)
		ctfs, err := ctftime.GetCTFs(10, onsite)
		if err != nil {
			log.Error("failed to get CTFs", "error", err)
			_ = fail("failed to get the CTFs from CTFtime.")
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
		firstStart := time.Time{}
		for _, ctf := range ctfs {
			startTime, err := time.Parse(time.RFC3339, ctf.Start)
			if err != nil {
				log.Warn("Unable to parse CTF start date", "ctf", ctf.Title, "start", ctf.Start, "error", err)
				continue
			}
			y, w := startTime.ISOWeek()
			if y == year && w == week {
				ctfsThisWeek = append(ctfsThisWeek, ctf)

				if firstStart.IsZero() || startTime.Before(firstStart) {
					firstStart = startTime
				}
			}
		}

		log.Info("CTFs found for this week", "count", len(ctfsThisWeek))

		if len(ctfsThisWeek) < 2 || len(ctfsThisWeek) > 10 {
			return fail("There must be between 2 and 10 CTFs this week to create a vote.")
		}

		pollTitle := "Vote the next CTF to participate in! 🎉"
		log.Info("Creating vote poll", "ctfs_count", len(ctfsThisWeek))

		const defaultDuration = 48
		hoursUntilFirstStart := int(time.Until(firstStart).Hours())

		if durationProvided {
			// If the user-provided duration would end after the first event starts, return an error.
			if duration >= hoursUntilFirstStart {
				return fail(fmt.Sprintf("The specified duration of %dh exceeds the time until the first CTF starts (%dh away). Please use a shorter duration.", duration, hoursUntilFirstStart))
			}
		} else {
			// Default: 48h, but if any event starts within 48h, end the poll 2h before it.
			if hoursUntilFirstStart <= defaultDuration {
				duration = hoursUntilFirstStart - 2
			} else {
				duration = defaultDuration
			}

			if duration < 1 {
				duration = max(1, hoursUntilFirstStart/2)
			}
		}

		answers := make([]discord.PollMedia, 0, len(ctfsThisWeek))
		for _, ctf := range ctfsThisWeek {
			text := fmt.Sprintf("%.40s (%d)", ctf.Title, ctf.ID)
			log.Debug("Adding CTF to poll options", "ctf", text)
			answers = append(answers, discord.PollMedia{Text: &text})
		}

		// Sent to the channel, not as a followup: the deferred response is ephemeral and the poll must be public.
		if _, err := e.Client().Rest.CreateMessage(e.Channel().ID(), discord.MessageCreate{
			Content: "It's time to vote for the next CTF! Cast your vote below: 🗳️",
			Poll: &discord.PollCreate{
				LayoutType:       discord.PollLayoutTypeDefault,
				AllowMultiselect: false,
				Question: discord.PollMedia{
					Text: &pollTitle,
				},
				Answers:  answers,
				Duration: duration,
			},
		}); err != nil {
			log.Error("failed to create poll", "error", err)
			_ = fail("failed to create the poll.")
			return err
		}

		_, err = e.CreateFollowupMessage(discord.MessageCreate{
			Content: "Poll created ✅",
			Flags:   discord.MessageFlagEphemeral,
		})
		return err
	}
}

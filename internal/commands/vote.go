package commands

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctftime"
)

const (
	// maxPollHours is the longest poll Discord accepts.
	maxPollHours = 768
	// defaultPollHours is the poll duration unless a CTF starts sooner.
	defaultPollHours = 48
)

var voteCommand = discord.SlashCommandCreate{
	Name:        "vote",
	Description: "Vote the next ctf to participate in",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionInt{
			Name:        "duration",
			Description: "Duration of the vote in hours (default 48h)",
			MinValue:    new(1),
			MaxValue:    new(maxPollHours),
		},
		discord.ApplicationCommandOptionBool{
			Name:        "onsite",
			Description: "Set to true to include also onsite CTFs (default false)",
		},
	},
}

func (h *Handlers) vote(e *handler.CommandEvent) error {
	options := e.SlashCommandInteractionData()
	duration, durationProvided := options.OptInt("duration")
	onsite, _ := options.OptBool("onsite")

	if durationProvided && (duration < 1 || duration > maxPollHours) {
		return reply(e, fmt.Sprintf("The duration must be between 1 and %d hours. ❌", maxPollHours))
	}

	ctfs, err := h.ctftime.Upcoming(e.Ctx, 10, onsite)
	if err != nil {
		log.Error("failed to get CTFs", "err", err)
		return reply(e, "Failed to get the CTFs from CTFtime. ❌")
	}

	candidates, firstStart := ctfsOfTheWeek(ctfs, time.Now())
	if len(candidates) < 2 {
		return reply(e, "There must be between 2 and 10 CTFs this week to create a vote. ❌")
	}

	hoursUntilFirstStart := int(time.Until(firstStart).Hours())
	if !durationProvided {
		duration = defaultPollDuration(hoursUntilFirstStart)
	} else if duration >= hoursUntilFirstStart {
		return reply(e, fmt.Sprintf("The specified duration of %dh exceeds the time until the first CTF starts (%dh away). Please use a shorter duration. ❌", duration, hoursUntilFirstStart))
	}

	answers := make([]discord.PollMedia, 0, len(candidates))
	for _, event := range candidates {
		text := fmt.Sprintf("%.40s (%d)", event.Title, event.ID)
		answers = append(answers, discord.PollMedia{Text: &text})
	}
	question := "Vote the next CTF to participate in! 🎉"

	// Sent to the channel, not as a followup: the deferred response is ephemeral and the poll must be public.
	if _, err := e.Client().Rest.CreateMessage(e.Channel().ID(), discord.MessageCreate{
		Content: "It's time to vote for the next CTF! Cast your vote below: 🗳️",
		Poll: &discord.PollCreate{
			LayoutType: discord.PollLayoutTypeDefault,
			Question:   discord.PollMedia{Text: &question},
			Answers:    answers,
			Duration:   duration,
		},
	}); err != nil {
		return err
	}
	return reply(e, "Poll created ✅")
}

// ctfsOfTheWeek returns the events starting in the week of now, or in the next
// one near the weekend, and the earliest of their start times.
func ctfsOfTheWeek(events []ctftime.Event, now time.Time) ([]ctftime.Event, time.Time) {
	if day := now.Weekday(); day == time.Saturday || day == time.Sunday {
		now = now.AddDate(0, 0, 7)
	}
	year, week := now.ISOWeek()

	var (
		matching []ctftime.Event
		first    time.Time
	)
	for _, event := range events {
		start, err := time.Parse(time.RFC3339, event.Start)
		if err != nil {
			log.Warn("Unable to parse CTF start date", "ctf", event.Title, "start", event.Start, "err", err)
			continue
		}
		if y, w := start.ISOWeek(); y != year || w != week {
			continue
		}
		matching = append(matching, event)
		if first.IsZero() || start.Before(first) {
			first = start
		}
	}
	return matching, first
}

// defaultPollDuration ends the poll 2h before the first CTF when that comes
// before the default, and is never shorter than 1h.
func defaultPollDuration(hoursUntilFirstStart int) int {
	if hoursUntilFirstStart > defaultPollHours {
		return defaultPollHours
	}
	if d := hoursUntilFirstStart - 2; d >= 1 {
		return d
	}
	return max(1, hoursUntilFirstStart/2)
}

package commands

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctftime"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/store"
)

var reportCommand = discord.SlashCommandCreate{
	Name:        "report",
	Description: "Generate a report for the current CTF.",
}

// reportMaxAge is how long results are reused before asking CTFtime again.
const reportMaxAge = 24 * time.Hour

func (h *Handlers) report(e *handler.CommandEvent) error {
	server, err := h.store.Server(*e.GuildID())
	if errors.Is(err, store.ErrNotFound) {
		return reply(e, notConfigured)
	}
	if err != nil {
		return err
	}

	c, ok, err := h.channelCTF(e, e.Channel().ID())
	if !ok {
		return err
	}
	if !participates(e, c) {
		return reply(e, "You must be participating in this CTF to see its report. ❌")
	}

	report, err := h.store.Report(c.ID)
	missing := errors.Is(err, store.ErrNotFound)
	if err != nil && !missing {
		return err
	}

	if missing || report.Place == -1 || report.Score == -1 || time.Since(report.UpdatedAt) >= reportMaxAge {
		results, err := h.ctftime.Results(e.Ctx, c.CTFTimeID, ctfYear(c.Name), server.TeamID)
		switch {
		case errors.Is(err, ctftime.ErrNoResults):
			// Not published yet: show what we have.
		case err != nil:
			log.Error("failed to fetch results from CTFtime", "err", err, "ctf_id", c.ID)
			return reply(e, "Failed to retrieve the report from CTFtime. ❌")
		default:
			report.CTFID = c.ID
			report.Place = results.Place
			report.Score = results.Score
			// CTFtime may not report solves: keep the count from /flag then.
			if results.Solves > 0 {
				report.Solves = results.Solves
			}
			if err := h.store.SaveReport(report); err != nil {
				log.Error("failed to save report", "err", err, "ctf_id", c.ID)
			}
		}
	}

	place, score := "N/A", "N/A"
	if report.Place > 0 {
		place, score = strconv.Itoa(report.Place), strconv.Itoa(report.Score)
	}
	now := time.Now()
	_, err = e.CreateFollowupMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Title: "Report for " + c.Name,
			Color: colorBlurple,
			Fields: []discord.EmbedField{
				{Name: "Place", Value: place},
				{Name: "Score", Value: score},
				{Name: "Solves", Value: strconv.Itoa(report.Solves)},
			},
			Timestamp: &now,
		}},
		Flags: discord.MessageFlagEphemeral,
	})
	return err
}

// ctfYear reads the year out of a CTF name, "Title - 2026", defaulting to the current one.
func ctfYear(name string) int {
	if _, suffix, found := strings.CutLast(name, "-"); found {
		if year, err := strconv.Atoi(strings.TrimSpace(suffix)); err == nil {
			return year
		}
	}
	return time.Now().Year()
}

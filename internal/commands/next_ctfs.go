package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctftime"
)

var nextCTFsCommand = discord.SlashCommandCreate{
	Name:        "next-ctfs",
	Description: "List the next ctfs on ctftime.",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionBool{
			Name:        "ephemeral",
			Description: "Whether the response should be ephemeral or not (default: True)",
		},
		discord.ApplicationCommandOptionInt{
			Name:        "limit",
			Description: "The maximum number of CTFs to display (default: 5, max:10)",
			MinValue:    new(1),
			MaxValue:    new(10),
		},
	},
}

func (h *Handlers) nextCTFs(e *handler.CommandEvent) error {
	options := e.SlashCommandInteractionData()
	hidden, ok := options.OptBool("ephemeral")
	if !ok {
		hidden = true
	}
	limit, ok := options.OptInt("limit")
	if !ok {
		limit = 5
	}

	if err := e.DeferCreateMessage(hidden); err != nil {
		return err
	}

	ctfs, err := h.ctftime.Upcoming(e.Ctx, limit, true)
	if err != nil {
		log.Error("failed to get CTFs", "err", err)
		_, err := e.CreateFollowupMessage(discord.MessageCreate{Content: "Failed to get CTFs."})
		return err
	}
	if len(ctfs) == 0 {
		_, err := e.CreateFollowupMessage(discord.MessageCreate{Content: "No CTFs found."})
		return err
	}

	lines := make([]string, 0, len(ctfs))
	for i, event := range ctfs[:min(limit, len(ctfs))] {
		lines = append(lines, nextCTFLine(i+1, event))
	}
	now := time.Now().UTC()
	_, err = e.CreateFollowupMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Title:       "📋 Next CTFs",
			Description: strings.Join(lines, "\n"),
			Color:       0x0000FF,
			Timestamp:   &now,
			Footer:      &discord.EmbedFooter{Text: fmt.Sprintf("Total: %d available CTFs", len(ctfs))},
		}},
	})
	return err
}

func nextCTFLine(index int, event ctftime.Event) string {
	start, err := time.Parse(time.RFC3339, event.Start)
	if err != nil {
		log.Warn("Unable to parse CTF start date", "ctf", event.Title, "start", event.Start, "err", err)
	}
	end := start.Add(time.Duration(event.Duration.Days)*24*time.Hour + time.Duration(event.Duration.Hours)*time.Hour)

	onsite := "🌐 Online"
	if event.Onsite {
		onsite = "🏢 Onsite"
	}
	formatEmoji := "📋"
	switch event.Format {
	case "Jeopardy":
		formatEmoji = "🎯"
	case "Attack-Defense":
		formatEmoji = "⚔️"
	case "Mixed":
		formatEmoji = "🔀"
	}

	const line = "### %d • %s\n" +
		"🆔 `%d` • ⚖️ **Weight:** `%g` • 📍 **Location:** %s\n" +
		"📅 **Start:** <t:%d:F> (<t:%d:R>) • 🗓️ **End:** <t:%d:F> (<t:%d:R>)\n" +
		" ⏱️ **Duration:** `%dd %dh`\n" +
		"%s **Format:** %s • 🔗 **Link:** [CTFtime](%s)\n"
	return fmt.Sprintf(line,
		index, event.Title, event.ID, event.Weight, onsite,
		start.Unix(), start.Unix(), end.Unix(), end.Unix(),
		event.Duration.Days, event.Duration.Hours, formatEmoji, event.Format, event.CtftimeURL)
}

package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctftime"
)

type CTF struct {
	ID         int
	Title      string
	Start      time.Time
	CTFTimeURL string
	Duration   struct {
		Days  int
		Hours int
	}
	Weight float64
	Onsite bool
	Format string
}

var nextCTFs = discord.SlashCommandCreate{
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
			MinValue:    &[]int{1}[0],
			MaxValue:    &[]int{10}[0],
		},
	},
}

func NextCTFsHandler() handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		options := e.SlashCommandInteractionData()
		ephemeral, ok := options.OptBool("ephemeral")
		if !ok {
			ephemeral = true
		}
		limit, ok := options.OptInt("limit")
		if !ok {
			limit = 5
		}

		err := e.DeferCreateMessage(ephemeral)
		if err != nil {
			return err
		}

		if e.GuildID() == nil {
			_, err = e.CreateFollowupMessage(discord.MessageCreate{
				Content: "This command can only be used in a guild.",
			})
			return err
		}

		ctfs, err := ctftime.GetCTFs(limit, true)
		if err != nil {
			_, err = e.CreateFollowupMessage(discord.MessageCreate{
				Content: "failed to get CTFs.",
			})
			return err
		}

		if len(ctfs) == 0 {
			_, err = e.CreateFollowupMessage(discord.MessageCreate{
				Content: "No CTFs found.",
			})
			return err
		}

		lines := []string{}
		for i, ctf := range ctfs[:min(limit, len(ctfs))] {
			idx := i + 1
			start := fmt.Sprintf("<t:%d:F> (<t:%d:R>)",
				func() int64 { t, _ := time.Parse(time.RFC3339, ctf.Start); return t.Unix() }(),
				func() int64 { t, _ := time.Parse(time.RFC3339, ctf.Start); return t.Unix() }(),
			)
			durationStr := fmt.Sprintf("%dd %dh", ctf.Duration.Days, ctf.Duration.Hours)
			onsite := "🌐 Online"
			if ctf.Onsite {
				onsite = "🏢 Onsite"
			}
			formatEmoji := "📋"
			switch ctf.Format {
			case "Jeopardy":
				formatEmoji = "🎯"
			case "Attack-Defense":
				formatEmoji = "⚔️"
			case "Mixed":
				formatEmoji = "🔀"
			}

			end := fmt.Sprintf("<t:%d:F> (<t:%d:R>)",
				func() int64 {
					t, _ := time.Parse(time.RFC3339, ctf.Start)
					return t.Add(time.Duration(ctf.Duration.Days)*24*time.Hour + time.Duration(ctf.Duration.Hours)*time.Hour).Unix()
				}(),
				func() int64 {
					t, _ := time.Parse(time.RFC3339, ctf.Start)
					return t.Add(time.Duration(ctf.Duration.Days)*24*time.Hour + time.Duration(ctf.Duration.Hours)*time.Hour).Unix()
				}(),
			)

			link := fmt.Sprintf("[CTFtime](%s)", ctf.CtftimeURL)
			line := fmt.Sprintf("### %d • %s\n🆔 `%d` • ⚖️ **Weight:** `%g` • 📍 **Location:** %s\n📅 **Start:** %s • 🗓️ **End:** %s\n ⏱️ **Duration:** `%s`\n%s **Format:** %s • 🔗 **Link:** %s\n",
				idx, ctf.Title, ctf.ID, ctf.Weight, onsite, start, end, durationStr, formatEmoji, ctf.Format, link)
			lines = append(lines, line)
		}

		now := time.Now().UTC()
		embed := discord.Embed{
			Title:       "📋 Next CTFs",
			Description: strings.Join(lines, "\n"),
			Color:       0x0000FF,
			Timestamp:   &now,
			Footer: &discord.EmbedFooter{
				Text: fmt.Sprintf("Total: %d available CTFs", len(ctfs)),
			},
		}

		_, err = e.CreateFollowupMessage(discord.MessageCreate{
			Embeds: []discord.Embed{embed},
		})
		return err
	}
}

package commands

import (
	"ctftime"
	"database"
	"discordutils"
	"models"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var report = discord.SlashCommandCreate{
	Name:        "report",
	Description: "Generate a report for the current CTF.",
}

func ReportHandler() handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		db := database.GetInstance().Connection()
		if err := e.DeferCreateMessage(true); err != nil {
			log.Error("failed to defer create message", "error", err)
			return err
		}

		if e.GuildID() == nil {
			log.Warn("Report command used outside of a guild", "user_id", e.User().ID)
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "This command can only be used inside a guild. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		var server models.Server
		err := server.GetByID(db, *e.GuildID())
		if err != nil {
			log.Error("failed to fetch server configuration", "error", err)
			return err
		}

		if server == (models.Server{}) {
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "The server is not configured. ❌ Please run the /init command.",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		// Find the CTF associated with the current channel
		var ctf models.CTF
		err = ctf.GetByChannelID(db, e.Channel().ID(), *e.GuildID())
		if err != nil {
			log.Error("failed to fetch CTF for channel", "error", err)
			_, sendErr := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "failed to find the CTF for this channel. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			if sendErr != nil {
				log.Error("failed to send followup", "error", sendErr)
				return sendErr
			}
			return nil
		}
		if ctf == (models.CTF{}) {
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "No CTF is associated with this channel. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		if !slices.Contains(e.Member().RoleIDs, ctf.RoleID) {
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "You must be participating in this CTF to see its report. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		var report models.Report
		err = report.GetByCTFID(db, ctf.ID)
		log.Debug("Fetched report from database", "report", report)
		if err != nil {
			log.Error("failed to fetch report", "error", err)
			_, sendErr := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "failed to retrieve the report. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			if sendErr != nil {
				log.Error("failed to send followup", "error", sendErr)
				return sendErr
			}
			return nil
		}

		nameParts := strings.Split(ctf.Name, "-")
		yearStr := strings.TrimSpace(nameParts[len(nameParts)-1])
		year, err := strconv.Atoi(yearStr)
		if err != nil {
			log.Error("failed to parse year from CTF name", "error", err)
			year = time.Now().Year()
		}

		// If report data is missing, fetch from CTFTime
		// Check if the report was updated in the last day
		isRecent := false
		if report != (models.Report{}) {
			if time.Since(report.UpdatedAt) < 24*time.Hour {
				isRecent = true
			}
		}
		if report == (models.Report{}) || report.Place == -1 || report.Score == -1 || !isRecent {
			log.Debug("Fetching from CTFTime", "ctf_id", ctf.ID)
			results, err := ctftime.GetResultsInfo(ctf.CTFTimeID, year, server.TeamID)
			if err != nil {
				log.Error("failed to fetch results from CTFTime", "error", err)
				_, sendErr := e.CreateFollowupMessage(discord.MessageCreate{
					Content: "failed to retrieve the report from CTFTime. ❌",
					Flags:   discord.MessageFlagEphemeral,
				})
				if sendErr != nil {
					log.Error("failed to send followup", "error", sendErr)
					return sendErr
				}
				return nil
			}
			if results != nil {
				report.CTFID = ctf.ID
				report.Place = results.Place
				report.Score = results.Score
				// CTFtime may not report solves: keep the count from /flag then.
				if results.Solves > 0 {
					report.Solves = results.Solves
				}
				err = report.Update(db)
				if err != nil {
					log.Error("failed to update report in database", "error", err)
				}
			}
		}

		place, score := "N/A", "N/A"
		if report.Place > 0 {
			place, score = strconv.Itoa(report.Place), strconv.Itoa(report.Score)
		}

		now := time.Now()
		embed := discord.Embed{
			Title: "Report for " + ctf.Name,
			Color: discordutils.ColorBlurple,
			Fields: []discord.EmbedField{
				{
					Name:  "Place",
					Value: place,
				},
				{
					Name:  "Score",
					Value: score,
				},
				{
					Name:  "Solves",
					Value: strconv.Itoa(report.Solves),
				},
			},
			Timestamp: &now,
		}

		_, err = e.CreateFollowupMessage(discord.MessageCreate{
			Embeds: []discord.Embed{embed},
			Flags:  discord.MessageFlagEphemeral,
		})
		return err
	}
}

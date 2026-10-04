package commands

import (
	"database"
	"models"
	"strconv"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var remove = discord.SlashCommandCreate{
	Name:        "remove",
	Description: "Remove a CTF event in the discord server.",
}

func ParseCtfsOptions(ctfs []models.CTF) []discord.StringSelectMenuOption {
	options := make([]discord.StringSelectMenuOption, 0, len(ctfs))
	for _, ctf := range ctfs {
		option := discord.NewStringSelectMenuOption(
			ctf.Name,
			strconv.Itoa(int(ctf.ID)),
		).WithDescription(ctf.Description)
		options = append(options, option)
	}
	return options
}

func RemoveHandler() handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		db := database.GetInstance().Connection()
		if err := e.DeferCreateMessage(true); err != nil {
			log.Error("failed to defer create message", "error", err)
			return err
		}

		if e.GuildID() == nil {
			log.Warn("Create command used outside of a guild", "user_id", e.User().ID)
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "This command can only be used inside a guild. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		ctfs, err := models.CTF{}.GetList(db, *e.GuildID())
		if err != nil {
			log.Error("failed to fetch ctfs list", "error", err)
			return err
		}

		if len(ctfs) > 25 {
			ctfs = ctfs[len(ctfs)-25:]
		}

		if len(ctfs) == 0 {
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "Non ci sono CTF da rimuovere. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		msg := discord.MessageCreate{
			Content: "Seleziona un CTF:",
			Components: []discord.LayoutComponent{
				discord.NewActionRow(
					discord.NewStringSelectMenu(
						"remove_ctf_select",
						"Seleziona le CTF da rimuovere",
						ParseCtfsOptions(ctfs)...,
					).WithPlaceholder("Nessun CTF selezionato"),
				),
			},
		}

		_, err = e.CreateFollowupMessage(msg)
		return err
	}
}

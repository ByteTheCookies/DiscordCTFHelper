package commands

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctf"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/store"
)

var (
	createCommand = discord.SlashCommandCreate{
		Name:        "create",
		Description: "Create a CTF event in the discord server.",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionInt{
				Name:        "ctftime_id",
				Description: "The ID of the ctf on ctftime",
				Required:    true,
			},
		},
	}
	removeCommand = discord.SlashCommandCreate{
		Name:        "remove",
		Description: "Remove a CTF event in the discord server.",
	}
)

func (h *Handlers) create(e *handler.CommandEvent) error {
	server, err := h.store.Server(*e.GuildID())
	if err != nil {
		return err
	}
	if server.ActiveCategoryID == 0 {
		return reply(e, "failed to set the category. ❌ Please check the configuration or contact support.")
	}

	err = h.ctf.Create(e.Ctx, e.Client(), server, e.SlashCommandInteractionData().Int("ctftime_id"))
	switch {
	case errors.Is(err, store.ErrExists):
		return reply(e, "The CTF is already present in the discord server. ❌")
	case errors.Is(err, ctf.ErrInvalidEvent):
		log.Error("failed to get CTF information", "err", err)
		return reply(e, "Failed to get the information of the CTF. ❌")
	case err != nil:
		return err
	}

	if server.RoleTeamID != 0 {
		if _, err := e.Client().Rest.CreateMessage(e.Channel().ID(), discord.MessageCreate{
			Content: fmt.Sprintf("<@&%s> New CTF published in <#%s> 🎉", server.RoleTeamID, server.FeedChannelID),
		}); err != nil {
			log.Error("failed to announce CTF to team role", "err", err)
		}
	}
	return reply(e, "CTF created in the discord server ✅")
}

// removeOptions builds the entries of the /remove menu.
func removeOptions(ctfs []store.CTF) []discord.StringSelectMenuOption {
	options := make([]discord.StringSelectMenuOption, 0, len(ctfs))
	for _, c := range ctfs {
		options = append(options, discord.NewStringSelectMenuOption(
			ctf.Truncate(c.Name, ctf.MaxOptionLabelLength),
			strconv.FormatInt(c.ID, 10),
		).WithDescription(ctf.Truncate(c.Description, ctf.MaxOptionDescLength)))
	}
	return options
}

func (h *Handlers) remove(e *handler.CommandEvent) error {
	ctfs, err := h.store.ListCTFs(*e.GuildID())
	if err != nil {
		return err
	}
	if len(ctfs) == 0 {
		return reply(e, "There are no CTFs to remove. ❌")
	}
	// A select menu holds at most 25 options.
	if len(ctfs) > 25 {
		ctfs = ctfs[len(ctfs)-25:]
	}

	_, err = e.CreateFollowupMessage(discord.MessageCreate{
		Content: "Select a CTF:",
		Components: []discord.LayoutComponent{
			discord.NewActionRow(
				discord.NewStringSelectMenu(
					"remove_ctf_select",
					"Select the CTF to remove",
					removeOptions(ctfs)...,
				),
			),
		},
		Flags: discord.MessageFlagEphemeral,
	})
	return err
}

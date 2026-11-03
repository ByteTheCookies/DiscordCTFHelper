package commands

import (
	"errors"
	"fmt"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/store"
)

var (
	credsCommand = discord.SlashCommandCreate{
		Name:        "creds",
		Description: "Setup creds for the ctf.",
	}
	deleteCredsCommand = discord.SlashCommandCreate{
		Name:        "delete-creds",
		Description: "Delete the creds for the ctf.",
	}
)

const credsRoleRequired = "You don't have the required role to view or set credentials. ❌"

// creds answers without deferring, because a deferred interaction can't open a modal.
func (h *Handlers) creds(e *handler.CommandEvent) error {
	c, err := h.store.CTFByChannel(e.Channel().ID(), *e.GuildID())
	if errors.Is(err, store.ErrNotFound) {
		return e.CreateMessage(ephemeral("No CTF is associated with this channel. ❌"))
	}
	if err != nil {
		return err
	}
	if !participates(e, c) {
		return e.CreateMessage(ephemeral(credsRoleRequired))
	}

	creds, err := h.store.Creds(c.ID)
	if errors.Is(err, store.ErrNotFound) {
		return e.Modal(credsModal(c.ID))
	}
	if err != nil {
		return err
	}

	personal := "No"
	if creds.Personal {
		personal = "Yes"
	}
	now := time.Now()
	return e.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Title:       "Credentials for " + c.Name,
			Description: fmt.Sprintf("**Username:** `%s`\n**Password:** `%s`\n**Need personal:** %s\n\n", creds.Username, creds.Password, personal),
			Color:       colorGreen,
			Timestamp:   &now,
		}},
		Flags: discord.MessageFlagEphemeral,
	})
}

func credsModal(ctfID int64) discord.ModalCreate {
	return discord.NewModalCreate(fmt.Sprintf("creds_modal_%d", ctfID), "Credentials Form").
		AddLabel("Username", discord.NewTextInput("username", discord.TextInputStyleShort).
			WithMinLength(1).
			WithMaxLength(20).
			WithPlaceholder("Enter the username").
			WithRequired(true)).
		AddLabel("Password", discord.NewTextInput("password", discord.TextInputStyleShort).
			WithMinLength(1).
			WithMaxLength(100).
			WithPlaceholder("Enter the password").
			WithRequired(true)).
		AddLabel("Personal", discord.NewTextInput("personal", discord.TextInputStyleShort).
			WithMinLength(2).
			WithMaxLength(3).
			WithPlaceholder("yes/no").
			WithRequired(true))
}

func (h *Handlers) deleteCreds(e *handler.CommandEvent) error {
	c, ok, err := h.channelCTF(e, e.Channel().ID())
	if !ok {
		return err
	}
	if !participates(e, c) {
		return reply(e, credsRoleRequired)
	}

	if _, err := h.store.Creds(c.ID); errors.Is(err, store.ErrNotFound) {
		return reply(e, "No credentials are available for this CTF. ❌")
	} else if err != nil {
		return err
	}
	if err := h.store.DeleteCreds(c.ID); err != nil {
		return err
	}
	return reply(e, "Credential removed correctly ✅")
}

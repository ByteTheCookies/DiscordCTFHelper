// Package commands implements the bot's slash commands.
package commands

import (
	"context"
	"errors"
	"slices"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctf"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctftime"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/store"
)

// Commands lists the slash commands to register on Discord.
var Commands = []discord.ApplicationCommandCreate{
	versionCommand,
	createCommand,
	removeCommand,
	flagCommand,
	deleteFlagCommand,
	reportCommand,
	credsCommand,
	deleteCredsCommand,
	nextCTFsCommand,
	initCommand,
	challCommand,
	voteCommand,
	pingCommand,
	helpCommand,
	guideCommand,
	rulesCommand,
}

// Handlers implements the commands on top of their dependencies.
type Handlers struct {
	store   *store.Store
	ctftime *ctftime.Client
	ctf     *ctf.Service
}

// New returns Handlers using the given dependencies.
func New(st *store.Store, ct *ctftime.Client, svc *ctf.Service) *Handlers {
	return &Handlers{store: st, ctftime: ct, ctf: svc}
}

// Router routes every command through the access checks it requires. Handlers
// receive ctx, which should end when the bot shuts down.
func (h *Handlers) Router(ctx context.Context) *handler.Mux {
	r := handler.New()
	r.DefaultContext(func() context.Context { return ctx })
	r.Error(onError)

	r.Command("/version", version)
	r.Group(func(r handler.Router) {
		r.Use(deferEphemeral)
		r.Command("/ping", ping)
		r.Command("/rules", rules)
		r.Command("/guide", guide)
		r.Command("/help", help)
	})
	// These two answer by themselves: /creds may open a modal, /next-ctfs may be public.
	r.Group(func(r handler.Router) {
		r.Use(guildOnly)
		r.Command("/creds", h.creds)
		r.Command("/next-ctfs", h.nextCTFs)
	})
	r.Group(func(r handler.Router) {
		r.Use(guildOnly, deferEphemeral)
		r.Command("/init", h.init)
		r.Command("/flag", h.flag)
		r.Command("/chall", h.chall)
		r.Command("/report", h.report)
		r.Command("/delete-creds", h.deleteCreds)
	})
	r.Group(func(r handler.Router) {
		r.Use(guildOnly, deferEphemeral, h.managerOnly)
		r.Command("/create", h.create)
		r.Command("/remove", h.remove)
		r.Command("/delete-flag", h.deleteFlag)
		r.Command("/vote", h.vote)
	})
	return r
}

func ephemeral(content string) discord.MessageCreate {
	return discord.MessageCreate{Content: content, Flags: discord.MessageFlagEphemeral}
}

// onError logs what a handler returned and tells the user something went wrong.
func onError(e *handler.InteractionEvent, err error) {
	log.Error("command failed", "err", err, "user_id", e.User().ID)
	msg := ephemeral("Something went wrong. ❌")
	if _, followupErr := e.CreateFollowupMessage(msg); followupErr != nil {
		// The interaction was not acknowledged yet.
		if createErr := e.CreateMessage(msg); createErr != nil {
			log.Error("failed to report command failure", "err", createErr)
		}
	}
}

// guildOnly stops commands used outside of a guild, so handlers can rely on the
// guild and the member being there.
func guildOnly(next handler.Handler) handler.Handler {
	return func(e *handler.InteractionEvent) error {
		if e.GuildID() == nil || e.Member() == nil {
			return e.CreateMessage(ephemeral("This command can only be used inside a guild. ❌"))
		}
		return next(e)
	}
}

// deferEphemeral acknowledges the interaction within Discord's 3 second limit.
// Handlers then answer with followup messages.
func deferEphemeral(next handler.Handler) handler.Handler {
	return func(e *handler.InteractionEvent) error {
		if err := e.DeferCreateMessage(true); err != nil {
			return err
		}
		return next(e)
	}
}

// managerOnly stops users without the manager role. It requires guildOnly and deferEphemeral.
func (h *Handlers) managerOnly(next handler.Handler) handler.Handler {
	return func(e *handler.InteractionEvent) error {
		server, err := h.store.Server(*e.GuildID())
		if errors.Is(err, store.ErrNotFound) {
			_, err := e.CreateFollowupMessage(ephemeral(notConfigured))
			return err
		}
		if err != nil {
			return err
		}
		if !server.IsManager(e.Member().RoleIDs) {
			_, err := e.CreateFollowupMessage(ephemeral(ctf.PermissionDenied))
			return err
		}
		return next(e)
	}
}

const notConfigured = "The server is not configured. ❌ Please run the /init command."

// reply answers a deferred interaction with a message only the user can see.
func reply(e *handler.CommandEvent, content string) error {
	_, err := e.CreateFollowupMessage(ephemeral(content))
	return err
}

// channelCTF returns the CTF of channelID. When there is none the user has been
// told and ok is false.
func (h *Handlers) channelCTF(e *handler.CommandEvent, channelID snowflake.ID) (c store.CTF, ok bool, err error) {
	c, err = h.store.CTFByChannel(channelID, *e.GuildID())
	if errors.Is(err, store.ErrNotFound) {
		return c, false, reply(e, "No CTF is associated with this channel. ❌")
	}
	return c, err == nil, err
}

// participates reports whether the user holds the role of the CTF.
func participates(e *handler.CommandEvent, c store.CTF) bool {
	return slices.Contains(e.Member().RoleIDs, c.RoleID)
}

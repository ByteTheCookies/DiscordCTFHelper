// Package listeners reacts to Discord events that are not slash commands.
package listeners

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctf"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/store"
)

// cookieEmojiID is the custom emoji used to greet new members.
const cookieEmojiID = 1365671553622343792

// Listeners implements the event listeners on top of their dependencies.
type Listeners struct {
	store *store.Store
	ctf   *ctf.Service
}

// New returns Listeners using the given dependencies.
func New(st *store.Store, svc *ctf.Service) *Listeners {
	return &Listeners{store: st, ctf: svc}
}

// All returns every event listener of the bot.
func (l *Listeners) All() []bot.EventListener {
	return []bot.EventListener{
		bot.NewListenerFunc(l.credsModal),
		bot.NewListenerFunc(l.removeSelect),
		bot.NewListenerFunc(l.pollResult),
		bot.NewListenerFunc(greetNewMember),
		bot.NewListenerFunc(l.reactionAdd),
		bot.NewListenerFunc(l.reactionRemove),
		bot.NewListenerFunc(l.scheduledEventUpdate),
	}
}

func (l *Listeners) credsModal(e *events.ModalSubmitInteractionCreate) {
	if err := e.CreateMessage(discord.MessageCreate{Content: l.saveCreds(e)}); err != nil {
		log.Error("failed to answer modal", "err", err)
	}
}

// saveCreds stores the submitted credentials and returns the answer for the user.
func (l *Listeners) saveCreds(e *events.ModalSubmitInteractionCreate) string {
	const failed = "Error processing modal submission."

	idText, isCreds := strings.CutPrefix(e.Data.CustomID, "creds_modal_")
	if !isCreds {
		return "Unknown modal submitted."
	}
	if e.GuildID() == nil {
		return failed
	}
	ctfID, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		log.Error("failed to parse CTF ID from modal ID", "err", err, "modal_id", e.Data.CustomID)
		return failed
	}

	c, err := l.store.CTFByChannel(e.Channel().ID(), *e.GuildID())
	if err != nil {
		log.Error("failed to fetch CTF", "err", err, "guild_id", *e.GuildID())
		return failed
	}
	if c.ID != ctfID {
		log.Error("CTF ID mismatch", "expected_ctf_id", c.ID, "modal_ctf_id", ctfID)
		return failed
	}

	creds := store.Creds{CTFID: ctfID}
	for component := range e.Data.AllComponents() {
		input, ok := component.(discord.TextInputComponent)
		if !ok {
			continue
		}
		switch input.CustomID {
		case "username":
			creds.Username = input.Value
		case "password":
			creds.Password = input.Value
		case "personal":
			creds.Personal = strings.EqualFold(input.Value, "yes")
		}
	}
	if err := l.store.AddCreds(creds); err != nil {
		log.Error("failed to save credentials", "err", err, "ctf_id", ctfID)
		return "Error saving credentials."
	}
	return fmt.Sprintf("<@&%s>, Credentials submitted ✅.", c.RoleID)
}

func greetNewMember(e *events.MessageCreate) {
	if e.Message.Type != discord.MessageTypeUserJoin || e.GuildID == nil {
		return
	}
	reaction := "🍪"
	emoji, err := e.Client().Rest.GetEmoji(*e.GuildID, cookieEmojiID)
	if err != nil {
		log.Debug("custom cookie emoji not available", "err", err)
	} else {
		reaction = emoji.String()
	}
	if err := e.Client().Rest.AddReaction(e.ChannelID, e.Message.ID, reaction); err != nil {
		log.Error("failed to react to new member", "err", err)
	}
}

func (l *Listeners) reactionAdd(e *events.MessageReactionAdd) {
	if e.GuildID == nil || e.Member == nil || e.Member.User.Bot {
		return
	}
	c, err := l.store.CTFByMessage(e.MessageID, *e.GuildID)
	if errors.Is(err, store.ErrNotFound) {
		return
	}
	if err != nil {
		log.Error("failed to fetch CTF by message", "err", err, "message_id", e.MessageID)
		return
	}
	if c.RoleID == 0 {
		return
	}
	if err := e.Client().Rest.AddMemberRole(*e.GuildID, e.Member.User.ID, c.RoleID); err != nil {
		log.Error("failed to add CTF role to member", "err", err, "role_id", c.RoleID, "user_id", e.Member.User.ID)
	}
}

func (l *Listeners) reactionRemove(e *events.MessageReactionRemove) {
	if e.GuildID == nil {
		return
	}
	c, err := l.store.CTFByMessage(e.MessageID, *e.GuildID)
	if errors.Is(err, store.ErrNotFound) {
		return
	}
	if err != nil {
		log.Error("failed to fetch CTF by message", "err", err, "message_id", e.MessageID)
		return
	}
	if c.RoleID == 0 {
		return
	}
	if err := e.Client().Rest.RemoveMemberRole(*e.GuildID, e.UserID, c.RoleID); err != nil {
		log.Error("failed to remove CTF role from member", "err", err, "role_id", c.RoleID, "user_id", e.UserID)
	}
}

func (l *Listeners) removeSelect(e *events.ComponentInteractionCreate) {
	if e.Data.CustomID() != "remove_ctf_select" || e.GuildID() == nil || e.Member() == nil {
		return
	}

	server, err := l.store.Server(*e.GuildID())
	if err != nil {
		log.Error("failed to fetch server configuration", "err", err, "guild_id", *e.GuildID())
	}
	if err != nil || !server.IsManager(e.Member().RoleIDs) {
		if err := e.CreateMessage(discord.MessageCreate{Content: ctf.PermissionDenied, Flags: discord.MessageFlagEphemeral}); err != nil {
			log.Error("failed to answer select menu", "err", err)
		}
		return
	}

	// Removal takes several Discord calls: acknowledge within the 3 second limit.
	if err := e.DeferCreateMessage(true); err != nil {
		log.Error("failed to defer select menu", "err", err)
		return
	}

	content := l.removeCTFs(e, e.StringSelectMenuInteractionData().Values)
	if _, err := e.Client().Rest.CreateFollowupMessage(e.ApplicationID(), e.Token(), discord.MessageCreate{
		Content: content,
		Flags:   discord.MessageFlagEphemeral,
	}); err != nil {
		log.Error("failed to send followup", "err", err)
	}
}

// removeCTFs removes the selected CTFs and returns the answer for the user.
func (l *Listeners) removeCTFs(e *events.ComponentInteractionCreate, values []string) string {
	for _, value := range values {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			log.Error("failed to parse CTF ID from select value", "err", err, "value", value)
			return "Error processing selection."
		}

		c, err := l.store.CTFByID(id)
		if errors.Is(err, store.ErrNotFound) || (err == nil && c.ServerID != *e.GuildID()) {
			return fmt.Sprintf("No CTF found with ID %d.", id)
		}
		if err != nil {
			log.Error("failed to fetch CTF", "err", err, "ctf_id", id)
			return "Error retrieving the CTF from the database."
		}

		if err := l.ctf.Remove(e.Client(), c); err != nil {
			log.Error("failed to remove CTF", "err", err, "ctf_id", id)
			return "Error while removing the CTF."
		}
	}
	return "CTF removed successfully ✅."
}

func (l *Listeners) scheduledEventUpdate(e *events.GuildScheduledEventUpdate) {
	status := e.GuildScheduled.Status
	if e.OldGuildScheduled.Status == status ||
		(status != discord.ScheduledEventStatusActive && status != discord.ScheduledEventStatusCompleted) {
		return
	}

	c, err := l.store.CTFByEvent(e.GuildScheduled.ID, e.GuildScheduled.GuildID)
	if errors.Is(err, store.ErrNotFound) {
		return
	}
	if err != nil {
		log.Error("failed to fetch CTF for scheduled event", "err", err, "event_id", e.GuildScheduled.ID)
		return
	}

	channel, err := e.Client().Rest.GetChannel(c.TextChannelID)
	if err != nil {
		log.Error("failed to fetch CTF channel", "err", err, "channel_id", c.TextChannelID)
		return
	}
	if channel.Type() != discord.ChannelTypeGuildText {
		return
	}

	var announcement string
	if status == discord.ScheduledEventStatusActive {
		announcement = fmt.Sprintf("<@&%s> The CTF has started! Good luck to all participants! :tada:", c.RoleID)
	} else {
		l.archive(e, c)
		announcement = fmt.Sprintf("<@&%s> The CTF **%s** has ended! The channel has been moved to the archived category.", c.RoleID, c.Name)
	}
	if _, err := e.Client().Rest.CreateMessage(c.TextChannelID, discord.MessageCreate{Content: announcement}); err != nil {
		log.Error("failed to announce CTF status", "err", err, "channel_id", c.TextChannelID)
	}
}

// archive moves the channel of an ended CTF to the archive and retires its role.
func (l *Listeners) archive(e *events.GuildScheduledEventUpdate, c store.CTF) {
	server, err := l.store.Server(c.ServerID)
	if err != nil {
		log.Error("failed to fetch server to archive CTF", "err", err, "server_id", c.ServerID)
	} else if server.ArchiveCategoryID != 0 {
		if _, err := e.Client().Rest.UpdateChannel(c.TextChannelID, discord.GuildTextChannelUpdate{
			ParentID: &server.ArchiveCategoryID,
			Position: new(0),
		}); err != nil {
			log.Error("failed to move channel to archive category", "err", err, "channel_id", c.TextChannelID)
		}
	}

	if c.RoleID == 0 {
		return
	}
	if _, err := e.Client().Rest.UpdateRole(c.ServerID, c.RoleID, discord.RoleUpdate{
		Color:       new(0xD3D3D3),
		Hoist:       new(false),
		Mentionable: new(false),
	}); err != nil {
		log.Error("failed to update CTF role", "err", err, "role_id", c.RoleID)
	}
}

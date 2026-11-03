package commands

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/store"
)

var (
	flagCommand = discord.SlashCommandCreate{
		Name:        "flag",
		Description: "Register a flag in the ctf.",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionString{
				Name:        "flag",
				Description: "The flag.",
				Required:    true,
			},
			discord.ApplicationCommandOptionUser{
				Name:        "mate",
				Description: "If you solved the challenge together with someone, specify their tag",
			},
			discord.ApplicationCommandOptionString{
				Name:        "challenge_name",
				Description: "the challenge name (optional)",
			},
		},
	}
	deleteFlagCommand = discord.SlashCommandCreate{
		Name:        "delete-flag",
		Description: "Delete a flag in the ctf.",
	}
)

func (h *Handlers) flag(e *handler.CommandEvent) error {
	threadID, channelID, err := resolveThread(e)
	if err != nil {
		return err
	}

	c, ok, err := h.channelCTF(e, channelID)
	if !ok {
		return err
	}

	if threadID != 0 {
		locked := true
		if _, err := e.Client().Rest.UpdateChannel(threadID, discord.GuildThreadUpdate{Locked: &locked}); err != nil {
			log.Error("failed to lock challenge thread", "err", err)
		}
	}

	solves, err := h.store.AddSolve(c.ID)
	if err != nil {
		// Losing the count must not hide the flag from the team.
		log.Error("failed to update report", "err", err, "ctf_id", c.ID)
	}

	msg, err := e.Client().Rest.CreateMessage(channelID, flagMessage(e, c))
	if err != nil {
		return err
	}
	if err := e.Client().Rest.AddReaction(channelID, msg.ID, "🔥"); err != nil {
		return err
	}
	return reply(e, fmt.Sprintf("Flag registered successfully! ✅\nTotal solves for this CTF: %d", solves))
}

// resolveThread tells whether the command runs in an active thread. It returns
// the thread, or 0, and the channel the CTF would be attached to.
func resolveThread(e *handler.CommandEvent) (threadID, channelID snowflake.ID, err error) {
	threads, err := e.Client().Rest.GetActiveGuildThreads(*e.GuildID())
	if err != nil {
		return 0, 0, fmt.Errorf("fetch active threads: %w", err)
	}
	for _, thread := range threads.Threads {
		if thread.ID() == e.Channel().ID() {
			return thread.ID(), *thread.ParentID(), nil
		}
	}
	return 0, e.Channel().ID(), nil
}

func flagMessage(e *handler.CommandEvent, c store.CTF) discord.MessageCreate {
	options := e.SlashCommandInteractionData()
	content := fmt.Sprintf("<@&%s> NEW FLAG FOUND BY %s", c.RoleID, e.User().Mention())
	// The flag and challenge name are user text: only ping who this message is meant for.
	mentions := discord.AllowedMentions{
		Parse: []discord.AllowedMentionType{},
		Roles: []snowflake.ID{c.RoleID},
		Users: []snowflake.ID{e.User().ID},
	}
	if mate, ok := options.OptMember("mate"); ok {
		content += " and " + mate.User.Mention()
		mentions.Users = append(mentions.Users, mate.User.ID)
	}
	if challenge, ok := options.OptString("challenge_name"); ok {
		content += " for challenge: " + challenge
	}
	content += fmt.Sprintf("🎉\n> `%s`", options.String("flag"))
	return discord.MessageCreate{Content: content, AllowedMentions: &mentions}
}

func (h *Handlers) deleteFlag(e *handler.CommandEvent) error {
	c, ok, err := h.channelCTF(e, e.Channel().ID())
	if !ok {
		return err
	}

	if err := h.store.RemoveSolve(c.ID); err != nil {
		return err
	}
	return reply(e, "Flag deleted successfully ✅")
}

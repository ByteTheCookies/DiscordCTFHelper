package ctfbot

import (
	"testing"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

// A slow listener must not keep the others waiting: this one only finishes once
// the next has run, which can't happen if events are dispatched one at a time.
func TestSlowListenerDoesNotBlockOthers(t *testing.T) {
	secondRan := make(chan struct{})
	finished := make(chan bool, 1)

	slow := bot.NewListenerFunc(func(*events.MessageCreate) {
		select {
		case <-secondRan:
			finished <- true
		case <-time.After(2 * time.Second):
			finished <- false
		}
	})
	second := bot.NewListenerFunc(func(*events.MessageCreate) { close(secondRan) })

	client, err := NewClient(t.Context(), "MTIzNDU2.fake.token", slow, second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close(t.Context()) })

	client.EventManager.DispatchEvent(&events.MessageCreate{})
	if !<-finished {
		t.Fatal("events are handled one at a time")
	}
}

type guildCommandsRest struct {
	rest.Rest
	guild    snowflake.ID
	commands []discord.ApplicationCommandCreate
}

func (f *guildCommandsRest) SetGuildCommands(_, guildID snowflake.ID, commands []discord.ApplicationCommandCreate, _ ...rest.RequestOpt) ([]discord.ApplicationCommand, error) {
	f.guild, f.commands = guildID, commands
	return nil, nil
}

func TestClearGuildCommandsSendsAnEmptyList(t *testing.T) {
	f := &guildCommandsRest{}
	clearGuildCommands(t.Context()).OnEvent(&events.GuildReady{GenericGuild: &events.GenericGuild{
		GenericEvent: events.NewGenericEvent(&bot.Client{Rest: f}, 0, 0),
		GuildID:      10,
	}})
	if f.guild != 10 {
		t.Fatalf("cleared guild %d", f.guild)
	}
	// nil would reach Discord as null and be rejected.
	if f.commands == nil || len(f.commands) != 0 {
		t.Fatalf("commands = %#v, want an empty non-nil list", f.commands)
	}
}

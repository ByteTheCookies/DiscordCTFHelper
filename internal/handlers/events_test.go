package handlers

import (
	"errors"
	"testing"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/database"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/models"
)

// eventRest records which channel the handler picked and stops it there.
type eventRest struct {
	rest.Rest
	channels []snowflake.ID
}

func (f *eventRest) GetChannel(id snowflake.ID, _ ...rest.RequestOpt) (discord.Channel, error) {
	f.channels = append(f.channels, id)
	return nil, errors.New("stop here")
}

func updateEvent(f *eventRest, id snowflake.ID, name string, from, to discord.ScheduledEventStatus) {
	ScheduleEventUpdateHandler().OnEvent(&events.GuildScheduledEventUpdate{
		GenericGuildScheduledEvent: &events.GenericGuildScheduledEvent{
			GenericEvent:   events.NewGenericEvent(&bot.Client{Rest: f}, 0, 0),
			GuildScheduled: discord.GuildScheduledEvent{ID: id, GuildID: guildID, Name: name, Status: to},
		},
		OldGuildScheduled: discord.GuildScheduledEvent{Status: from},
	})
}

// A CTF held every year shares its event name with last year's edition.
func TestScheduledEventUpdateFindsCTFByEventID(t *testing.T) {
	db := database.GetInstance().Connection()
	lastYear := models.CTF{Name: "FooCTF - 2025", ServerID: guildID, TextChannelID: 2025, EventID: 1025}
	thisYear := models.CTF{Name: "FooCTF - 2026", ServerID: guildID, TextChannelID: 2026, EventID: 1026}
	if err := errors.Join(lastYear.Add(db), thisYear.Add(db)); err != nil {
		t.Fatal(err)
	}

	f := &eventRest{}
	updateEvent(f, 1026, "FooCTF", discord.ScheduledEventStatusScheduled, discord.ScheduledEventStatusActive)
	if len(f.channels) != 1 || f.channels[0] != 2026 {
		t.Fatalf("handler used channels %v, want this year's", f.channels)
	}
}

func TestScheduledEventUpdateIgnoresEditsWithoutStatusChange(t *testing.T) {
	db := database.GetInstance().Connection()
	ctf := models.CTF{Name: t.Name(), ServerID: guildID, TextChannelID: 3000, EventID: 1030}
	if err := ctf.Add(db); err != nil {
		t.Fatal(err)
	}

	f := &eventRest{}
	updateEvent(f, 1030, t.Name(), discord.ScheduledEventStatusActive, discord.ScheduledEventStatusActive)
	if len(f.channels) != 0 {
		t.Fatalf("an edit announced the CTF again: %v", f.channels)
	}
}

func TestScheduledEventUpdateIgnoresUnrelatedEvents(t *testing.T) {
	f := &eventRest{}
	updateEvent(f, 4040, "FooCTF", discord.ScheduledEventStatusScheduled, discord.ScheduledEventStatusActive)
	if len(f.channels) != 0 {
		t.Fatalf("an event of no CTF touched channels %v", f.channels)
	}
}

func TestPollHandlerIgnoresOtherMessages(t *testing.T) {
	// No client and no guild: reaching either would panic.
	VotePollHandler().OnEvent(&events.MessageCreate{GenericMessage: &events.GenericMessage{
		Message: discord.Message{Type: discord.MessageTypeDefault},
	}})
}

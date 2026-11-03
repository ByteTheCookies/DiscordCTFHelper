package listeners

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctf"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctftime"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/store"
)

const (
	guildID     = 10
	managerRole = 100
)

func setup(t *testing.T) (*Listeners, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.ReplaceServer(store.Server{ID: guildID, RoleManagerID: managerRole}); err != nil {
		t.Fatal(err)
	}
	return New(st, ctf.NewService(st, ctftime.New(""))), st
}

func addCTF(t *testing.T, st *store.Store, c store.CTF) store.CTF {
	t.Helper()
	if c.Name == "" {
		c.Name = t.Name()
	}
	if err := st.AddCTF(&c); err != nil {
		t.Fatal(err)
	}
	return c
}

func exists(t *testing.T, st *store.Store, id int64) bool {
	t.Helper()
	_, err := st.CTFByID(id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	return err == nil
}

// removeRest panics on every Discord call a test does not expect: the CTFs in
// these tests have no role, event or feed message.
type removeRest struct {
	rest.Rest
	reached   bool
	deleteErr error
	replies   []string
}

func (f *removeRest) DeleteChannel(_ snowflake.ID, _ ...rest.RequestOpt) error {
	f.reached = true
	return f.deleteErr
}

func (f *removeRest) CreateFollowupMessage(_ snowflake.ID, _ string, m discord.MessageCreate, _ ...rest.RequestOpt) (*discord.Message, error) {
	f.replies = append(f.replies, m.Content)
	return &discord.Message{}, nil
}

func selectCTF(t *testing.T, l *Listeners, f *removeRest, ctfID int64, role int) string {
	t.Helper()
	raw := fmt.Sprintf(`{"id":"1","type":3,"application_id":"2","token":"t","version":1,"guild_id":"%d",
		"channel":{"id":"20","type":0,"name":"c","guild_id":"%d"},
		"member":{"user":{"id":"30","username":"u"},"roles":["%d"],"permissions":"0"},
		"message":{"id":"5","channel_id":"20","author":{"id":"2","username":"b"}},
		"data":{"component_type":3,"custom_id":"remove_ctf_select","values":["%d"]}}`,
		guildID, guildID, role, ctfID)
	var i discord.ComponentInteraction
	if err := json.Unmarshal([]byte(raw), &i); err != nil {
		t.Fatal(err)
	}

	var answer string
	l.removeSelect(&events.ComponentInteractionCreate{
		GenericEvent:         events.NewGenericEvent(&bot.Client{Rest: f}, 0, 0),
		ComponentInteraction: i,
		Respond: func(_ discord.InteractionResponseType, data discord.InteractionResponseData, _ ...rest.RequestOpt) error {
			answer = data.(discord.MessageCreate).Content
			return nil
		},
	})
	if len(f.replies) > 0 {
		return f.replies[len(f.replies)-1]
	}
	return answer
}

func TestRemoveSelectRejectsNonManagers(t *testing.T) {
	l, st := setup(t)
	c := addCTF(t, st, store.CTF{ServerID: guildID, TextChannelID: 20})
	f := &removeRest{}
	if answer := selectCTF(t, l, f, c.ID, 999); !strings.Contains(answer, "required role") {
		t.Fatalf("answer = %q", answer)
	}
	if f.reached || !exists(t, st, c.ID) {
		t.Fatal("a non-manager reached the removal")
	}
}

func TestRemoveSelectRemovesForManagers(t *testing.T) {
	l, st := setup(t)
	c := addCTF(t, st, store.CTF{ServerID: guildID, TextChannelID: 20})
	f := &removeRest{}
	if answer := selectCTF(t, l, f, c.ID, managerRole); !strings.Contains(answer, "removed successfully") {
		t.Fatalf("answer = %q", answer)
	}
	if !f.reached || exists(t, st, c.ID) {
		t.Fatal("the CTF was not removed")
	}
}

func TestRemoveSelectToleratesMissingChannel(t *testing.T) {
	l, st := setup(t)
	c := addCTF(t, st, store.CTF{ServerID: guildID, TextChannelID: 20})
	selectCTF(t, l, &removeRest{deleteErr: &rest.Error{Code: rest.JSONErrorCodeUnknownChannel}}, c.ID, managerRole)
	if exists(t, st, c.ID) {
		t.Fatal("a CTF whose channel is gone must still be removable")
	}
}

func TestRemoveSelectRemovesCTFWithoutChannel(t *testing.T) {
	l, st := setup(t)
	c := addCTF(t, st, store.CTF{ServerID: guildID}) // left behind by a failed /create
	selectCTF(t, l, &removeRest{}, c.ID, managerRole)
	if exists(t, st, c.ID) {
		t.Fatal("a half-created CTF must be removable")
	}
}

func TestRemoveSelectKeepsCTFWhenDiscordFails(t *testing.T) {
	l, st := setup(t)
	c := addCTF(t, st, store.CTF{ServerID: guildID, TextChannelID: 20})
	f := &removeRest{deleteErr: errors.New("discord is down")}
	if answer := selectCTF(t, l, f, c.ID, managerRole); !strings.Contains(answer, "Error while removing") {
		t.Fatalf("answer = %q", answer)
	}
	if !exists(t, st, c.ID) {
		t.Fatal("the CTF was dropped although its channel still exists")
	}
}

func TestRemoveSelectRejectsCTFsOfOtherGuilds(t *testing.T) {
	l, st := setup(t)
	c := addCTF(t, st, store.CTF{ServerID: 99, TextChannelID: 20})
	f := &removeRest{}
	selectCTF(t, l, f, c.ID, managerRole)
	if f.reached || !exists(t, st, c.ID) {
		t.Fatal("a manager of another guild reached the removal")
	}
}

// eventRest records which channel the handler picked and stops it there.
type eventRest struct {
	rest.Rest
	channels []snowflake.ID
}

func (f *eventRest) GetChannel(id snowflake.ID, _ ...rest.RequestOpt) (discord.Channel, error) {
	f.channels = append(f.channels, id)
	return nil, errors.New("stop here")
}

func updateEvent(l *Listeners, f *eventRest, id snowflake.ID, name string, from, to discord.ScheduledEventStatus) {
	l.scheduledEventUpdate(&events.GuildScheduledEventUpdate{
		GenericGuildScheduledEvent: &events.GenericGuildScheduledEvent{
			GenericEvent:   events.NewGenericEvent(&bot.Client{Rest: f}, 0, 0),
			GuildScheduled: discord.GuildScheduledEvent{ID: id, GuildID: guildID, Name: name, Status: to},
		},
		OldGuildScheduled: discord.GuildScheduledEvent{Status: from},
	})
}

// A CTF held every year shares its event name with last year's edition.
func TestScheduledEventUpdateFindsCTFByEventID(t *testing.T) {
	l, st := setup(t)
	addCTF(t, st, store.CTF{Name: "FooCTF - 2025", ServerID: guildID, TextChannelID: 2025, EventID: 1025})
	addCTF(t, st, store.CTF{Name: "FooCTF - 2026", ServerID: guildID, TextChannelID: 2026, EventID: 1026})

	f := &eventRest{}
	updateEvent(l, f, 1026, "FooCTF", discord.ScheduledEventStatusScheduled, discord.ScheduledEventStatusActive)
	if len(f.channels) != 1 || f.channels[0] != 2026 {
		t.Fatalf("handler used channels %v, want this year's", f.channels)
	}
}

func TestScheduledEventUpdateIgnoresEditsWithoutStatusChange(t *testing.T) {
	l, st := setup(t)
	addCTF(t, st, store.CTF{ServerID: guildID, TextChannelID: 3000, EventID: 1030})

	f := &eventRest{}
	updateEvent(l, f, 1030, t.Name(), discord.ScheduledEventStatusActive, discord.ScheduledEventStatusActive)
	if len(f.channels) != 0 {
		t.Fatalf("an edit announced the CTF again: %v", f.channels)
	}
}

func TestScheduledEventUpdateIgnoresUnrelatedEvents(t *testing.T) {
	l, _ := setup(t)
	f := &eventRest{}
	updateEvent(l, f, 4040, "FooCTF", discord.ScheduledEventStatusScheduled, discord.ScheduledEventStatusActive)
	if len(f.channels) != 0 {
		t.Fatalf("an event of no CTF touched channels %v", f.channels)
	}
}

func TestPollResultIgnoresOtherMessages(t *testing.T) {
	// No client and no guild: reaching either would panic.
	(&Listeners{}).pollResult(&events.MessageCreate{GenericMessage: &events.GenericMessage{
		Message: discord.Message{Type: discord.MessageTypeDefault},
	}})
}

func poll(counts map[int]int, answers ...string) discord.Poll {
	p := discord.Poll{Results: &discord.PollResults{}}
	for i, a := range answers {
		p.Answers = append(p.Answers, discord.PollAnswer{PollMedia: discord.PollMedia{Text: &a}})
		p.Results.AnswerCounts = append(p.Results.AnswerCounts, discord.PollAnswerCount{ID: i + 1, Count: counts[i+1]})
	}
	return p
}

func TestPollWinner(t *testing.T) {
	got, err := pollWinner(poll(map[int]int{1: 1, 2: 3}, "a", "b"))
	if err != nil || got != "b" {
		t.Errorf("winner = %q, err %v", got, err)
	}
	if _, err := pollWinner(poll(map[int]int{1: 2, 2: 2}, "a", "b")); !errors.Is(err, errPollTie) {
		t.Errorf("tie err = %v", err)
	}
	if _, err := pollWinner(poll(nil, "a", "b")); err == nil || errors.Is(err, errPollTie) {
		t.Errorf("no votes err = %v", err)
	}
}

func TestParseCTFTimeID(t *testing.T) {
	tests := map[string]int{"Foo (12)": 12, "Foo (bar) ( 34 )": 34}
	for in, want := range tests {
		if got, err := parseCTFTimeID(in); err != nil || got != want {
			t.Errorf("parse(%q) = %d, err %v", in, got, err)
		}
	}
	for _, in := range []string{"Foo", "Foo )1(", "Foo (x)"} {
		if _, err := parseCTFTimeID(in); err == nil {
			t.Errorf("parse(%q) accepted", in)
		}
	}
}

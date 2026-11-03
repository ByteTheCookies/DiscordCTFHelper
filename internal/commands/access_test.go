package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctf"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctftime"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/store"
)

const (
	guildID     = 10
	ctfChannel  = 20
	otherChan   = 21
	ctfThread   = 40
	otherThread = 41
	managerRole = 100
	ctfRole     = 200
	teamID      = 42
)

// testHandlers is what run() executes. Tests swap it to reach a fake CTFtime.
var (
	testStore    *store.Store
	testHandlers *Handlers
)

func newHandlers(ctftimeURL string) *Handlers {
	ct := ctftime.New(ctftimeURL)
	return New(testStore, ct, ctf.NewService(testStore, ct))
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ctfhelper")
	if err != nil {
		panic(err)
	}
	testStore, err = store.Open(dir + "/test.db")
	if err != nil {
		panic(err)
	}
	// Unreachable: a test that calls CTFtime without stubbing it gets an error.
	testHandlers = newHandlers("http://127.0.0.1:0")

	seed := store.CTF{Name: "ctf - 2026", ServerID: guildID, TextChannelID: ctfChannel, RoleID: ctfRole, EventID: 300}
	err = errors.Join(
		testStore.ReplaceServer(store.Server{ID: guildID, RoleManagerID: managerRole, ActiveCategoryID: 1, FeedChannelID: 2, TeamID: teamID}),
		testStore.AddCTF(&seed),
	)
	if err == nil {
		err = testStore.SaveReport(store.Report{CTFID: seed.ID, Place: 3, Score: 100, Solves: 2})
	}
	if err != nil {
		panic(err)
	}

	code := m.Run()
	_ = testStore.Close()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// fakeRest panics on any call it does not implement, so a handler that goes
// past its access check blows up the test.
type fakeRest struct {
	rest.Rest
	status   discord.ScheduledEventStatus
	eventErr error

	followups []discord.MessageCreate
	threads   []snowflake.ID
	locked    []snowflake.ID
	messages  []discord.MessageCreate
}

func (f *fakeRest) CreateFollowupMessage(_ snowflake.ID, _ string, m discord.MessageCreate, _ ...rest.RequestOpt) (*discord.Message, error) {
	f.followups = append(f.followups, m)
	return &discord.Message{}, nil
}

func (f *fakeRest) GetGuildScheduledEvent(_, _ snowflake.ID, _ bool, _ ...rest.RequestOpt) (*discord.GuildScheduledEvent, error) {
	if f.eventErr != nil {
		return nil, f.eventErr
	}
	return &discord.GuildScheduledEvent{Status: f.status}, nil
}

func (f *fakeRest) CreateMessage(_ snowflake.ID, m discord.MessageCreate, _ ...rest.RequestOpt) (*discord.Message, error) {
	f.messages = append(f.messages, m)
	return &discord.Message{ID: 500}, nil
}

func (f *fakeRest) CreateThreadFromMessage(_, msgID snowflake.ID, _ discord.ThreadCreateFromMessage, _ ...rest.RequestOpt) (*discord.GuildThread, error) {
	f.threads = append(f.threads, msgID)
	return nil, nil
}

func (*fakeRest) AddReaction(_, _ snowflake.ID, _ string, _ ...rest.RequestOpt) error { return nil }

func (f *fakeRest) UpdateChannel(id snowflake.ID, _ discord.ChannelUpdate, _ ...rest.RequestOpt) (discord.Channel, error) {
	f.locked = append(f.locked, id)
	return nil, nil
}

func (*fakeRest) GetActiveGuildThreads(_ snowflake.ID, _ ...rest.RequestOpt) (*discord.GuildActiveThreads, error) {
	raw := fmt.Sprintf(`{"threads":[
		{"id":"%d","type":11,"guild_id":"%d","parent_id":"%d","name":"ctf"},
		{"id":"%d","type":11,"guild_id":"%d","parent_id":"%d","name":"other"}
	],"members":[]}`, ctfThread, guildID, ctfChannel, otherThread, guildID, otherChan)
	var threads discord.GuildActiveThreads
	if err := json.Unmarshal([]byte(raw), &threads); err != nil {
		return nil, err
	}
	return &threads, nil
}

// run routes a slash command through the real router, access checks included,
// as a member holding roles and returns every message shown to that user.
func run(t *testing.T, f *fakeRest, command string, channel int, options string, roles ...int) []string {
	t.Helper()
	ids := make([]string, 0, len(roles))
	for _, r := range roles {
		ids = append(ids, strconv.Quote(strconv.Itoa(r)))
	}
	raw := fmt.Sprintf(`{"id":"1","type":2,"application_id":"2","token":"t","version":1,"guild_id":"%d",
		"channel":{"id":"%d","type":0,"name":"c","guild_id":"%d"},
		"member":{"user":{"id":"30","username":"u"},"roles":[%s],"permissions":"0"},
		"data":{"id":"3","name":%q,"type":1,"options":[%s]}}`,
		guildID, channel, guildID, strings.Join(ids, ","), command, options)
	i, err := discord.UnmarshalInteraction([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}

	var replies []string
	router := testHandlers.Router(t.Context())
	router.Error(func(_ *handler.InteractionEvent, err error) { t.Fatalf("command failed: %v", err) })
	router.OnEvent(&events.InteractionCreate{
		GenericEvent: events.NewGenericEvent(&bot.Client{Rest: f}, 0, 0),
		Interaction:  i,
		Respond: func(_ discord.InteractionResponseType, data discord.InteractionResponseData, _ ...rest.RequestOpt) error {
			if m, ok := data.(discord.MessageCreate); ok && m.Content != "" {
				replies = append(replies, m.Content)
			}
			return nil
		},
	})
	for _, m := range f.followups {
		replies = append(replies, m.Content)
	}
	return replies
}

func wantReply(t *testing.T, replies []string, want string) {
	t.Helper()
	if !slices.ContainsFunc(replies, func(r string) bool { return strings.Contains(r, want) }) {
		t.Fatalf("replies %q do not contain %q", replies, want)
	}
}

// The original bug: CheckPermission told the user "no" and let the command run anyway.
func TestManagerCommandsRejectOtherMembers(t *testing.T) {
	for _, command := range []string{"create", "delete-flag", "remove", "vote"} {
		t.Run(command, func(t *testing.T) {
			f := &fakeRest{}
			// A CTF participant is not a manager.
			replies := run(t, f, command, ctfChannel, "", ctfRole)
			wantReply(t, replies, "required role")
			// Anything after the refusal means the command kept running.
			if len(replies) != 1 || len(f.messages)+len(f.threads) != 0 {
				t.Fatalf("command acted anyway: replies %q, %+v", replies, f)
			}
		})
	}
}

func TestGuildCommandsRejectDirectMessages(t *testing.T) {
	raw := `{"id":"1","type":2,"application_id":"2","token":"t","version":1,
		"channel":{"id":"5","type":1},"user":{"id":"30","username":"u"},
		"data":{"id":"3","name":"remove","type":1}}`
	i, err := discord.UnmarshalInteraction([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	var reply string
	testHandlers.Router(t.Context()).OnEvent(&events.InteractionCreate{
		GenericEvent: events.NewGenericEvent(&bot.Client{Rest: &fakeRest{}}, 0, 0),
		Interaction:  i,
		Respond: func(_ discord.InteractionResponseType, data discord.InteractionResponseData, _ ...rest.RequestOpt) error {
			reply = data.(discord.MessageCreate).Content
			return nil
		},
	})
	if !strings.Contains(reply, "inside a guild") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestManagerCommandsNeedConfiguredServer(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/empty.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	old := testHandlers
	t.Cleanup(func() { testHandlers = old })
	ct := ctftime.New("http://127.0.0.1:0")
	testHandlers = New(st, ct, ctf.NewService(st, ct))

	wantReply(t, run(t, &fakeRest{}, "create", ctfChannel, "", managerRole), "/init")
}

func TestManagerCanUseRemove(t *testing.T) {
	replies := run(t, &fakeRest{}, "remove", ctfChannel, "", managerRole)
	wantReply(t, replies, "Select a CTF")
}

func TestChallAccess(t *testing.T) {
	const name = `{"name":"name","type":3,"value":"pwn1"}`
	unknownEvent := &rest.Error{Code: rest.JSONErrorCodeUnknownGuildScheduledEvent}

	tests := []struct {
		name       string
		channel    int
		roles      []int
		status     discord.ScheduledEventStatus
		eventErr   error
		want       string
		wantThread bool
	}{
		{name: "participant of the active CTF", channel: ctfChannel, roles: []int{ctfRole}, status: discord.ScheduledEventStatusActive, want: "Thread created", wantThread: true},
		{name: "not a participant", channel: ctfChannel, status: discord.ScheduledEventStatusActive, want: "participating"},
		{name: "manager who is not a participant", channel: ctfChannel, roles: []int{managerRole}, status: discord.ScheduledEventStatusActive, want: "participating"},
		{name: "outside the CTF channel", channel: otherChan, roles: []int{ctfRole}, status: discord.ScheduledEventStatusActive, want: "No CTF"},
		{name: "not started yet", channel: ctfChannel, roles: []int{ctfRole}, status: discord.ScheduledEventStatusScheduled, want: "not currently active"},
		{name: "finished", channel: ctfChannel, roles: []int{ctfRole}, status: discord.ScheduledEventStatusCompleted, want: "not currently active"},
		{name: "event gone from Discord", channel: ctfChannel, roles: []int{ctfRole}, eventErr: unknownEvent, want: "not currently active"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeRest{status: tt.status, eventErr: tt.eventErr}
			wantReply(t, run(t, f, "chall", tt.channel, name, tt.roles...), tt.want)
			if got := len(f.threads) == 1; got != tt.wantThread {
				t.Fatalf("thread created = %v, want %v", got, tt.wantThread)
			}
		})
	}
}

func TestReportAccess(t *testing.T) {
	tests := []struct {
		name    string
		channel int
		roles   []int
		denied  string
	}{
		{name: "not a participant", channel: ctfChannel, denied: "participating"},
		{name: "manager who is not a participant", channel: ctfChannel, roles: []int{managerRole}, denied: "participating"},
		{name: "outside the CTF channel", channel: otherChan, roles: []int{ctfRole}, denied: "No CTF"},
		{name: "participant in the CTF channel", channel: ctfChannel, roles: []int{ctfRole}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeRest{}
			replies := run(t, f, "report", tt.channel, "", tt.roles...)
			if tt.denied != "" {
				wantReply(t, replies, tt.denied)
				return
			}
			if len(f.followups) != 1 || len(f.followups[0].Embeds) != 1 {
				t.Fatalf("expected the report embed, got %+v", f.followups)
			}
		})
	}
}

func TestFlagLocksOnlyCTFThreads(t *testing.T) {
	const flagOpt = `{"name":"flag","type":3,"value":"flag{x}"}`

	f := &fakeRest{}
	wantReply(t, run(t, f, "flag", otherThread, flagOpt), "No CTF")
	if len(f.locked) != 0 {
		t.Fatalf("locked a thread outside any CTF: %v", f.locked)
	}

	f = &fakeRest{}
	run(t, f, "flag", ctfThread, flagOpt)
	if !slices.Equal(f.locked, []snowflake.ID{ctfThread}) {
		t.Fatalf("locked = %v, want the CTF thread", f.locked)
	}
}

func TestFlagCannotPingArbitraryTargets(t *testing.T) {
	const opts = `{"name":"flag","type":3,"value":"@everyone <@&999>"},{"name":"challenge_name","type":3,"value":"@here"}`

	f := &fakeRest{}
	run(t, f, "flag", ctfChannel, opts)
	if len(f.messages) != 1 {
		t.Fatalf("messages = %+v", f.messages)
	}
	m := f.messages[0].AllowedMentions
	if m == nil || m.Parse == nil || len(m.Parse) != 0 {
		t.Fatalf("mentions must not be parsed from the text: %+v", m)
	}
	if !slices.Equal(m.Roles, []snowflake.ID{ctfRole}) || !slices.Equal(m.Users, []snowflake.ID{30}) {
		t.Fatalf("unexpected ping targets: %+v", m)
	}
}

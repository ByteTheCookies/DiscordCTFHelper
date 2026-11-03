package listeners

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/store"
)

const botID = 2

// fakeRest panics on any call a test does not expect, and records the others.
type fakeRest struct {
	rest.Rest
	emoji       *discord.Emoji
	emojiErr    error
	poll        *discord.Poll
	reactions   []string
	rolesAdded  []snowflake.ID
	rolesGone   []snowflake.ID
	messages    []string
	moved       []snowflake.ID
	rolesEdited []snowflake.ID
}

func (f *fakeRest) GetEmoji(_, _ snowflake.ID, _ ...rest.RequestOpt) (*discord.Emoji, error) {
	return f.emoji, f.emojiErr
}

func (f *fakeRest) AddReaction(_, _ snowflake.ID, emoji string, _ ...rest.RequestOpt) error {
	f.reactions = append(f.reactions, emoji)
	return nil
}

func (f *fakeRest) AddMemberRole(_, _, roleID snowflake.ID, _ ...rest.RequestOpt) error {
	f.rolesAdded = append(f.rolesAdded, roleID)
	return nil
}

func (f *fakeRest) RemoveMemberRole(_, _, roleID snowflake.ID, _ ...rest.RequestOpt) error {
	f.rolesGone = append(f.rolesGone, roleID)
	return nil
}

func (*fakeRest) GetChannel(id snowflake.ID, _ ...rest.RequestOpt) (discord.Channel, error) {
	var c discord.UnmarshalChannel
	raw := fmt.Sprintf(`{"id":"%d","type":0,"name":"c","guild_id":"%d"}`, id, guildID)
	if err := c.UnmarshalJSON([]byte(raw)); err != nil {
		return nil, err
	}
	return c.Channel, nil
}

func (f *fakeRest) CreateMessage(_ snowflake.ID, m discord.MessageCreate, _ ...rest.RequestOpt) (*discord.Message, error) {
	f.messages = append(f.messages, m.Content)
	return &discord.Message{}, nil
}

func (f *fakeRest) UpdateChannel(_ snowflake.ID, update discord.ChannelUpdate, _ ...rest.RequestOpt) (discord.Channel, error) {
	f.moved = append(f.moved, *update.(discord.GuildTextChannelUpdate).ParentID)
	return nil, nil
}

func (f *fakeRest) UpdateRole(_, roleID snowflake.ID, _ discord.RoleUpdate, _ ...rest.RequestOpt) (*discord.Role, error) {
	f.rolesEdited = append(f.rolesEdited, roleID)
	return nil, nil
}

func (f *fakeRest) GetMessage(_, _ snowflake.ID, _ ...rest.RequestOpt) (*discord.Message, error) {
	return &discord.Message{Poll: f.poll}, nil
}

func client(f *fakeRest) *bot.Client {
	caches := cache.New()
	caches.SetSelfUser(discord.OAuth2User{ID: botID})
	return &bot.Client{Rest: f, Caches: caches}
}

func messageEvent(f *fakeRest, m discord.Message) *events.MessageCreate {
	guild := snowflake.ID(guildID)
	return &events.MessageCreate{GenericMessage: &events.GenericMessage{
		GenericEvent: events.NewGenericEvent(client(f), 0, 0),
		Message:      m,
		GuildID:      &guild,
	}}
}

func TestGreetNewMember(t *testing.T) {
	join := discord.Message{Type: discord.MessageTypeUserJoin}
	custom := discord.Emoji{ID: 7, Name: "cookie"}

	t.Run("custom emoji", func(t *testing.T) {
		f := &fakeRest{emoji: &custom}
		(&Listeners{greetEmojiID: 7}).greetNewMember(messageEvent(f, join))
		if len(f.reactions) != 1 || f.reactions[0] != custom.String() {
			t.Fatalf("reactions = %q", f.reactions)
		}
	})
	t.Run("custom emoji unavailable", func(t *testing.T) {
		f := &fakeRest{emojiErr: errors.New("unknown emoji")}
		(&Listeners{greetEmojiID: 7}).greetNewMember(messageEvent(f, join))
		if !slices.Equal(f.reactions, []string{"🍪"}) {
			t.Fatalf("reactions = %q", f.reactions)
		}
	})
	t.Run("not configured", func(t *testing.T) {
		f := &fakeRest{}
		(&Listeners{}).greetNewMember(messageEvent(f, join))
		if !slices.Equal(f.reactions, []string{"🍪"}) {
			t.Fatalf("reactions = %q", f.reactions)
		}
	})
	t.Run("other messages", func(t *testing.T) {
		f := &fakeRest{}
		(&Listeners{}).greetNewMember(messageEvent(f, discord.Message{Type: discord.MessageTypeDefault}))
		if len(f.reactions) != 0 {
			t.Fatalf("reactions = %q", f.reactions)
		}
	})
}

func reaction(f *fakeRest, messageID snowflake.ID) *events.GenericReaction {
	guild := snowflake.ID(guildID)
	return &events.GenericReaction{
		GenericEvent: events.NewGenericEvent(client(f), 0, 0),
		UserID:       30,
		MessageID:    messageID,
		GuildID:      &guild,
	}
}

func TestReactionsGrantAndRemoveTheCTFRole(t *testing.T) {
	l, st := setup(t)
	addCTF(t, st, store.CTF{ServerID: guildID, MsgID: 500, RoleID: 200})
	member := &discord.Member{User: discord.User{ID: 30}}
	automated := &discord.Member{User: discord.User{ID: 31, Bot: true}}

	f := &fakeRest{}
	l.reactionAdd(&events.MessageReactionAdd{GenericReaction: reaction(f, 500), Member: member})
	l.reactionAdd(&events.MessageReactionAdd{GenericReaction: reaction(f, 500), Member: automated})
	l.reactionAdd(&events.MessageReactionAdd{GenericReaction: reaction(f, 501), Member: member})
	if !slices.Equal(f.rolesAdded, []snowflake.ID{200}) {
		t.Fatalf("roles added = %v, want only the member's on the feed message", f.rolesAdded)
	}

	l.reactionRemove(&events.MessageReactionRemove{GenericReaction: reaction(f, 500)})
	l.reactionRemove(&events.MessageReactionRemove{GenericReaction: reaction(f, 501)})
	if !slices.Equal(f.rolesGone, []snowflake.ID{200}) {
		t.Fatalf("roles removed = %v", f.rolesGone)
	}
}

func modalEvent(t *testing.T, customID string, channel int, answer *string) *events.ModalSubmitInteractionCreate {
	t.Helper()
	input := func(id, value string) string {
		return fmt.Sprintf(`{"type":18,"id":1,"component":{"type":4,"id":2,"custom_id":%q,"value":%q}}`, id, value)
	}
	raw := fmt.Sprintf(`{"id":"1","type":5,"application_id":"2","token":"t","version":1,"guild_id":"%d",
		"channel":{"id":"%d","type":0,"name":"c","guild_id":"%d"},
		"member":{"user":{"id":"30","username":"u"},"roles":[],"permissions":"0"},
		"data":{"custom_id":%q,"components":[%s,%s,%s]}}`,
		guildID, channel, guildID, customID,
		input("username", "alice"), input("password", "s3cret"), input("personal", "YES"))
	i, err := discord.UnmarshalInteraction([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return &events.ModalSubmitInteractionCreate{
		GenericEvent:           events.NewGenericEvent(&bot.Client{}, 0, 0),
		ModalSubmitInteraction: i.(discord.ModalSubmitInteraction),
		Respond: func(_ discord.InteractionResponseType, data discord.InteractionResponseData, _ ...rest.RequestOpt) error {
			*answer = data.(discord.MessageCreate).Content
			return nil
		},
	}
}

func TestCredsModal(t *testing.T) {
	l, st := setup(t)
	c := addCTF(t, st, store.CTF{ServerID: guildID, TextChannelID: 20, RoleID: 200})
	id := fmt.Sprintf("creds_modal_%d", c.ID)
	var answer string

	l.credsModal(modalEvent(t, "other_modal", 20, &answer))
	if !strings.Contains(answer, "Unknown modal") {
		t.Errorf("unknown modal answer = %q", answer)
	}
	// The modal of a CTF submitted from the channel of another must not store anything.
	other := addCTF(t, st, store.CTF{Name: "other", ServerID: guildID, TextChannelID: 21, RoleID: 201})
	l.credsModal(modalEvent(t, id, 21, &answer))
	if !strings.Contains(answer, "Error") {
		t.Errorf("wrong channel answer = %q", answer)
	}
	for _, ctfID := range []int64{c.ID, other.ID} {
		if _, err := st.Creds(ctfID); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("credentials stored from the wrong channel for CTF %d: %v", ctfID, err)
		}
	}

	l.credsModal(modalEvent(t, id, 20, &answer))
	if !strings.Contains(answer, "submitted ✅") {
		t.Errorf("answer = %q", answer)
	}
	got, err := st.Creds(c.ID)
	if err != nil || got.Username != "alice" || got.Password != "s3cret" || !got.Personal {
		t.Fatalf("credentials = %+v, err %v", got, err)
	}

	l.credsModal(modalEvent(t, id, 20, &answer))
	if !strings.Contains(answer, "already submitted") {
		t.Errorf("second submission answer = %q", answer)
	}
}

func statusChange(f *fakeRest, eventID snowflake.ID, to discord.ScheduledEventStatus) *events.GuildScheduledEventUpdate {
	return &events.GuildScheduledEventUpdate{
		GenericGuildScheduledEvent: &events.GenericGuildScheduledEvent{
			GenericEvent:   events.NewGenericEvent(client(f), 0, 0),
			GuildScheduled: discord.GuildScheduledEvent{ID: eventID, GuildID: guildID, Status: to},
		},
		OldGuildScheduled: discord.GuildScheduledEvent{Status: discord.ScheduledEventStatusScheduled},
	}
}

func TestScheduledEventStartAnnounces(t *testing.T) {
	l, st := setup(t)
	addCTF(t, st, store.CTF{ServerID: guildID, TextChannelID: 20, RoleID: 200, EventID: 1000})

	f := &fakeRest{}
	l.scheduledEventUpdate(statusChange(f, 1000, discord.ScheduledEventStatusActive))
	if len(f.messages) != 1 || !strings.Contains(f.messages[0], "has started") {
		t.Fatalf("messages = %q", f.messages)
	}
	if len(f.moved)+len(f.rolesEdited) != 0 {
		t.Fatal("a starting CTF was archived")
	}
}

func TestScheduledEventEndArchives(t *testing.T) {
	l, st := setup(t)
	if err := st.ReplaceServer(store.Server{ID: guildID, RoleManagerID: managerRole, ArchiveCategoryID: 77}); err != nil {
		t.Fatal(err)
	}
	addCTF(t, st, store.CTF{ServerID: guildID, TextChannelID: 20, RoleID: 200, EventID: 1000})

	f := &fakeRest{}
	l.scheduledEventUpdate(statusChange(f, 1000, discord.ScheduledEventStatusCompleted))
	if !slices.Equal(f.moved, []snowflake.ID{77}) {
		t.Errorf("channel moved to %v", f.moved)
	}
	if !slices.Equal(f.rolesEdited, []snowflake.ID{200}) {
		t.Errorf("roles edited = %v", f.rolesEdited)
	}
	if len(f.messages) != 1 || !strings.Contains(f.messages[0], "has ended") {
		t.Errorf("messages = %q", f.messages)
	}
}

func pollResult(f *fakeRest) *events.MessageCreate {
	original := snowflake.ID(600)
	return messageEvent(f, discord.Message{
		Type:             discord.MessageTypePollResult,
		Author:           discord.User{ID: botID},
		MessageReference: &discord.MessageReference{MessageID: &original},
	})
}

func TestPollResult(t *testing.T) {
	tests := []struct {
		name string
		poll discord.Poll
		want string
	}{
		{name: "tie", poll: poll(map[int]int{1: 2, 2: 2}, "A (1)", "B (2)"), want: "tie"},
		// The fake CTFtime is unreachable: reaching the failure proves the winner was parsed and created.
		{name: "winner", poll: poll(map[int]int{1: 1, 2: 3}, "A (1)", "B (2)"), want: "failed to create"},
		{name: "no votes", poll: poll(nil, "A (1)", "B (2)")},
		{name: "winner without ID", poll: poll(map[int]int{1: 1}, "no id")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, _ := setup(t)
			f := &fakeRest{poll: &tt.poll}
			l.pollResult(pollResult(f))
			if tt.want == "" {
				if len(f.messages) != 0 {
					t.Fatalf("messages = %q", f.messages)
				}
				return
			}
			if len(f.messages) != 1 || !strings.Contains(f.messages[0], tt.want) {
				t.Fatalf("messages = %q, want %q", f.messages, tt.want)
			}
		})
	}
}

func TestPollResultIgnoresPollsOfOtherAuthors(t *testing.T) {
	l, _ := setup(t)
	tie := poll(map[int]int{1: 2, 2: 2}, "A (1)", "B (2)")
	f := &fakeRest{poll: &tie}
	e := pollResult(f)
	e.Message.Author.ID = 999
	l.pollResult(e)
	if len(f.messages) != 0 {
		t.Fatalf("acted on a poll the bot didn't create: %q", f.messages)
	}
}

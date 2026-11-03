package ctf

import (
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctftime"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/store"
)

// fakeRest rejects every event carrying an image; rejectAll rejects the others too.
type fakeRest struct {
	rest.Rest
	rejectAll bool
	acceptAll bool
	images    []*discord.Icon
	descs     []string
	overwrite []discord.PermissionOverwrite
}

func (f *fakeRest) CreateGuildScheduledEvent(_ snowflake.ID, c discord.GuildScheduledEventCreate, _ ...rest.RequestOpt) (*discord.GuildScheduledEvent, error) {
	f.images = append(f.images, c.Image)
	f.descs = append(f.descs, c.Description)
	if !f.acceptAll && (c.Image != nil || f.rejectAll) {
		return nil, &rest.Error{Message: "Unsupported image type given"}
	}
	return &discord.GuildScheduledEvent{ID: 400, Name: c.Name}, nil
}

func (*fakeRest) CreateRole(snowflake.ID, discord.RoleCreate, ...rest.RequestOpt) (*discord.Role, error) {
	return &discord.Role{ID: 200}, nil
}

func unmarshalChannel(raw string) (discord.Channel, error) {
	var c discord.UnmarshalChannel
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return nil, err
	}
	return c.Channel, nil
}

func (*fakeRest) GetChannel(snowflake.ID, ...rest.RequestOpt) (discord.Channel, error) {
	return unmarshalChannel(`{"id":"1","type":4,"name":"active","guild_id":"10"}`)
}

func (f *fakeRest) CreateGuildChannel(_ snowflake.ID, c discord.GuildChannelCreate, _ ...rest.RequestOpt) (discord.GuildChannel, error) {
	f.overwrite = c.(discord.GuildTextChannelCreate).PermissionOverwrites
	ch, err := unmarshalChannel(`{"id":"300","type":0,"name":"c","guild_id":"10"}`)
	if err != nil {
		return nil, err
	}
	return ch.(discord.GuildChannel), nil
}

func (*fakeRest) CreateMessage(snowflake.ID, discord.MessageCreate, ...rest.RequestOpt) (*discord.Message, error) {
	return &discord.Message{ID: 500}, nil
}

func (*fakeRest) PinMessage(_, _ snowflake.ID, _ ...rest.RequestOpt) error { return nil }

func (*fakeRest) AddReaction(_, _ snowflake.ID, _ string, _ ...rest.RequestOpt) error { return nil }

func serveLogo(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = png.Encode(w, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newService(t *testing.T, ctftimeURL string) (*Service, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewService(st, ctftime.New(ctftimeURL)), st
}

func TestCreateEventRetriesOnceWithoutImage(t *testing.T) {
	logo := serveLogo(t)
	s, _ := newService(t, "")
	info := ctftime.Event{Title: "ctf", Logo: logo.URL}
	now := time.Now()

	f := &fakeRest{}
	event, err := s.createEvent(t.Context(), &bot.Client{Rest: f}, 1, info, "desc", now, now)
	if err != nil || event.Name != "ctf" {
		t.Fatalf("event = %+v, err = %v", event, err)
	}
	if len(f.images) != 2 || f.images[0] == nil || f.images[1] != nil {
		t.Fatalf("expected image then no image, got %v", f.images)
	}
	if got := f.images[0].Type; got != discord.IconTypePNG {
		t.Fatalf("icon type = %q, want %q", got, discord.IconTypePNG)
	}

	f = &fakeRest{rejectAll: true}
	_, err = s.createEvent(t.Context(), &bot.Client{Rest: f}, 1, info, "desc", now, now)
	if _, ok := errors.AsType[*rest.Error](err); !ok {
		t.Fatalf("expected rest error, got %v", err)
	}
	if len(f.images) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(f.images))
	}
}

func TestCreateEventSurvivesUnavailableLogo(t *testing.T) {
	logo := httptest.NewServer(http.NotFoundHandler())
	defer logo.Close()
	s, _ := newService(t, "")
	now := time.Now()

	f := &fakeRest{}
	event, err := s.createEvent(t.Context(), &bot.Client{Rest: f}, 1, ctftime.Event{Title: "ctf", Logo: logo.URL}, "desc", now, now)
	if err != nil || event.Name != "ctf" {
		t.Fatalf("event = %+v, err = %v", event, err)
	}
	if len(f.images) != 1 || f.images[0] != nil {
		t.Fatalf("expected one attempt without image, got %v", f.images)
	}
}

func eventJSON(t *testing.T, description string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 7, "title": "FooCTF", "start": "2030-01-02T10:00:00Z", "finish": "2030-01-03T10:00:00Z",
			"url": "https://foo.example", "description": description,
		})
	}
}

func TestCreateStoresEverything(t *testing.T) {
	srv := httptest.NewServer(eventJSON(t, "short description"))
	defer srv.Close()
	s, st := newService(t, srv.URL)
	f := &fakeRest{acceptAll: true}
	server := store.Server{ID: 10, ActiveCategoryID: 1, RoleManagerID: 100, FeedChannelID: 2}

	if err := s.Create(t.Context(), &bot.Client{Rest: f}, server, 7); err != nil {
		t.Fatal(err)
	}

	got, err := st.CTFByChannel(300, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "FooCTF - 2030" || got.RoleID != 200 || got.MsgID != 500 || got.EventID != 400 || got.CTFTimeID != 7 {
		t.Fatalf("stored CTF = %+v", got)
	}
	if got.Description != "short description" || f.descs[0] != "short description" {
		t.Fatalf("description lost: stored %q, sent %q", got.Description, f.descs[0])
	}

	// @everyone, whose role ID is the guild ID, must not see the channel.
	everyone := f.overwrite[0].(discord.RolePermissionOverwrite)
	if everyone.RoleID != 10 || everyone.Deny != discord.PermissionViewChannel {
		t.Fatalf("@everyone overwrite = %+v", everyone)
	}

	if err := s.Create(t.Context(), &bot.Client{Rest: f}, server, 7); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("second create err = %v", err)
	}
}

func TestCreateRejectsUnusableEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":7,"title":"no dates"}`))
	}))
	defer srv.Close()
	s, _ := newService(t, srv.URL)

	// Any Discord call would panic on the nil fake.
	err := s.Create(t.Context(), &bot.Client{}, store.Server{ID: 10}, 7)
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("err = %v", err)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"exactly 10", 10, "exactly 10"},
		{"longer than ten", 10, "longer ..."},
		{"èèèèèèèèèèèè", 10, "èèèèèèè..."},
	}
	for _, tt := range tests {
		if got := Truncate(tt.in, tt.max); got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}

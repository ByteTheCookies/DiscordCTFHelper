package discordutils

import (
	"ctftime"
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
)

// fakeRest rejects every event carrying an image; rejectAll rejects the others too.
type fakeRest struct {
	rest.Rest
	rejectAll bool
	images    []*discord.Icon
}

func (f *fakeRest) CreateGuildScheduledEvent(_ snowflake.ID, c discord.GuildScheduledEventCreate, _ ...rest.RequestOpt) (*discord.GuildScheduledEvent, error) {
	f.images = append(f.images, c.Image)
	if c.Image != nil || f.rejectAll {
		return nil, &rest.Error{Message: "Unsupported image type given"}
	}
	return &discord.GuildScheduledEvent{Name: c.Name}, nil
}

func TestCreateEventsRetriesOnceWithoutImage(t *testing.T) {
	logo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = png.Encode(w, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	}))
	defer logo.Close()

	guildID := snowflake.ID(1)
	info := ctftime.Event{Title: "ctf", Logo: logo.URL}
	now := time.Now()

	f := &fakeRest{}
	event, err := CreateEvents(&bot.Client{Rest: f}, &guildID, info, "desc", now, now)
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
	_, err = CreateEvents(&bot.Client{Rest: f}, &guildID, info, "desc", now, now)
	var restErr *rest.Error
	if !errors.As(err, &restErr) {
		t.Fatalf("expected rest error, got %v", err)
	}
	if len(f.images) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(f.images))
	}
}

func TestCreateEventsSurvivesUnavailableLogo(t *testing.T) {
	logo := httptest.NewServer(http.NotFoundHandler())
	defer logo.Close()

	guildID := snowflake.ID(1)
	now := time.Now()
	f := &fakeRest{}
	event, err := CreateEvents(&bot.Client{Rest: f}, &guildID, ctftime.Event{Title: "ctf", Logo: logo.URL}, "desc", now, now)
	if err != nil || event.Name != "ctf" {
		t.Fatalf("event = %+v, err = %v", event, err)
	}
	if len(f.images) != 1 || f.images[0] != nil {
		t.Fatalf("expected one attempt without image, got %v", f.images)
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

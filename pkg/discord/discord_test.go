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

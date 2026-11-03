// Package ctf creates and removes CTFs: their database record and everything
// they own on Discord.
package ctf

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctftime"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/store"
)

// PermissionDenied is shown to users who lack the manager role.
const PermissionDenied = "You don't have the required role to run this command. ❌"

// Length limits enforced by Discord, in characters.
const (
	MaxOptionLabelLength = 100
	MaxOptionDescLength  = 100
	maxEventDescLength   = 1000
)

const embedColor = 0xBEBEFE

var (
	// ErrAlreadyExists is returned when the server already has the CTF.
	ErrAlreadyExists = errors.New("CTF already exists")
	// ErrInvalidEvent is returned when CTFtime has no usable data for the CTF.
	ErrInvalidEvent = errors.New("invalid CTF information")
)

// Service manages the lifecycle of CTFs.
type Service struct {
	store   *store.Store
	ctftime *ctftime.Client
}

// NewService returns a Service using the given dependencies.
func NewService(st *store.Store, ct *ctftime.Client) *Service {
	return &Service{store: st, ctftime: ct}
}

// Truncate shortens s to at most maxLen characters without splitting one.
func Truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen-3]) + "..."
}

// IsActive reports whether the CTF's scheduled event is running right now.
func IsActive(client *bot.Client, c store.CTF) (bool, error) {
	if c.EventID == 0 {
		return false, nil
	}
	event, err := client.Rest.GetGuildScheduledEvent(c.ServerID, c.EventID, false)
	if isRESTCode(err, rest.JSONErrorCodeUnknownGuildScheduledEvent) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("fetch scheduled event: %w", err)
	}
	return event.Status == discord.ScheduledEventStatusActive, nil
}

func isRESTCode(err error, code rest.JSONErrorCode) bool {
	var restErr *rest.Error
	return errors.As(err, &restErr) && restErr.Code == code
}

// Create publishes the CTFtime event on the server: role, channel, feed message
// and scheduled event.
func (s *Service) Create(ctx context.Context, client *bot.Client, server store.Server, ctftimeID int) error {
	info, err := s.ctftime.Event(ctx, ctftimeID)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidEvent, err)
	}
	start, end, err := eventTimes(info)
	if err != nil {
		return err
	}

	title := fmt.Sprintf("%s - %d", info.Title, start.Year())
	exists, err := s.store.CTFExists(title, server.ID)
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyExists
	}

	// Save the record early and after each step, so progress is not lost.
	c := store.CTF{Name: title, ServerID: server.ID, CTFTimeID: int64(ctftimeID)}
	if err := s.store.AddCTF(&c); err != nil {
		return err
	}
	log.Info("Creating CTF", "title", title)

	role, err := createRole(client, server.ID, title)
	if err != nil {
		return err
	}
	c.RoleID = role.ID
	if err := s.store.SaveCTF(&c); err != nil {
		return err
	}

	channel, err := createChannel(client, server, title, role.ID)
	if err != nil {
		return err
	}
	c.TextChannelID = channel.ID()
	if err := s.store.SaveCTF(&c); err != nil {
		return err
	}

	if err := welcome(client, channel.ID(), role, title, info.URL); err != nil {
		return err
	}

	msg, err := publish(client, server.FeedChannelID, info, start, end)
	if err != nil {
		return err
	}
	c.MsgID = msg.ID
	c.Description = Truncate(info.Description, maxEventDescLength)
	if err := s.store.SaveCTF(&c); err != nil {
		return err
	}

	// Discord rejects events that start in the past.
	if earliest := time.Now().Add(time.Minute); start.Before(earliest) {
		start = earliest
	}
	event, err := s.createEvent(ctx, client, server.ID, info, c.Description, start, end)
	if err != nil {
		return err
	}
	c.EventID = event.ID
	if err := s.store.SaveCTF(&c); err != nil {
		return err
	}

	log.Info("CTF created", "title", title)
	return nil
}

func eventTimes(info ctftime.Event) (start, end time.Time, err error) {
	start, err = time.Parse(time.RFC3339, info.Start)
	if err != nil {
		return start, end, fmt.Errorf("%w: start time: %w", ErrInvalidEvent, err)
	}
	end, err = time.Parse(time.RFC3339, info.Finish)
	if err != nil {
		return start, end, fmt.Errorf("%w: finish time: %w", ErrInvalidEvent, err)
	}
	return start, end, nil
}

func createRole(client *bot.Client, guildID snowflake.ID, name string) (*discord.Role, error) {
	color := rand.IntN(0xFFFFFF) //nolint:gosec // Not security sensitive.
	for color == embedColor {
		color = rand.IntN(0xFFFFFF) //nolint:gosec // Not security sensitive.
	}
	role, err := client.Rest.CreateRole(guildID, discord.RoleCreate{
		Name:        name,
		Color:       color,
		Mentionable: true,
		Hoist:       true,
	})
	if err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}
	return role, nil
}

func createChannel(client *bot.Client, server store.Server, name string, roleID snowflake.ID) (discord.GuildChannel, error) {
	category, err := client.Rest.GetChannel(server.ActiveCategoryID)
	if err != nil {
		return nil, fmt.Errorf("fetch category: %w", err)
	}
	if category.Type() != discord.ChannelTypeGuildCategory {
		return nil, fmt.Errorf("channel %s is not a category", server.ActiveCategoryID)
	}

	channel, err := client.Rest.CreateGuildChannel(server.ID, discord.GuildTextChannelCreate{
		Name:     name,
		ParentID: server.ActiveCategoryID,
		PermissionOverwrites: []discord.PermissionOverwrite{
			discord.RolePermissionOverwrite{
				RoleID: server.ID, // The @everyone role has the ID of the guild.
				Deny:   discord.PermissionViewChannel,
			},
			discord.RolePermissionOverwrite{
				RoleID: roleID,
				Allow:  discord.PermissionViewChannel | discord.PermissionSendMessages,
			},
			discord.RolePermissionOverwrite{
				RoleID: server.RoleManagerID,
				Deny:   discord.PermissionViewChannel,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}
	return channel, nil
}

func welcome(client *bot.Client, channelID snowflake.ID, role *discord.Role, title, url string) error {
	if _, err := client.Rest.CreateMessage(channelID, discord.MessageCreate{
		Content: fmt.Sprintf("%s Welcome to the CTF **%s**! 🎉", role.Mention(), title),
	}); err != nil {
		return fmt.Errorf("send welcome message: %w", err)
	}
	link, err := client.Rest.CreateMessage(channelID, discord.MessageCreate{Content: "Link to ctf: " + url})
	if err != nil {
		return fmt.Errorf("send link message: %w", err)
	}
	if err := client.Rest.PinMessage(channelID, link.ID); err != nil {
		log.Error("failed to pin message", "err", err)
	}
	return nil
}

func publish(client *bot.Client, feedChannelID snowflake.ID, info ctftime.Event, start, end time.Time) (*discord.Message, error) {
	description := fmt.Sprintf(`
**Description:**

%s

- **Start Time:** <t:%d:f>
- **End Time:** <t:%d:f>
- **URL:** %s
- **Format:** %s
- **Location:** %s
- **Weight:** %f
- **Prizes:**
%s

`, Truncate(info.Description, 2500), start.Unix(), end.Unix(), info.URL, info.Format, info.Location, info.Weight, Truncate(info.Prizes, 1000))

	now := time.Now()
	message, err := client.Rest.CreateMessage(feedChannelID, discord.MessageCreate{
		Embeds: []discord.Embed{{
			Title:       info.Title,
			Description: description,
			URL:         info.URL,
			Timestamp:   &now,
			Color:       embedColor,
			Thumbnail:   &discord.EmbedResource{URL: info.Logo},
			Footer:      &discord.EmbedFooter{Text: "Add a reaction to get the ctf role (only if you want to participate). 🙃"},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("send feed message: %w", err)
	}
	if err := client.Rest.AddReaction(feedChannelID, message.ID, "✅"); err != nil {
		return nil, fmt.Errorf("add feed reaction: %w", err)
	}
	return message, nil
}

func (s *Service) createEvent(ctx context.Context, client *bot.Client, guildID snowflake.ID, info ctftime.Event, description string, start, end time.Time) (*discord.GuildScheduledEvent, error) {
	var image *discord.Icon
	if info.Logo != "" {
		data, format, err := s.ctftime.Logo(ctx, info.Logo)
		if err != nil {
			// A missing or unreadable logo must not cost the whole event.
			log.Warn("failed to fetch the CTF logo, creating event without image", "logo", info.Logo, "err", err)
		} else {
			image = &discord.Icon{Data: data, Type: discord.IconType("image/" + format)}
		}
	}

	eventCreate := discord.GuildScheduledEventCreate{
		Name:               info.Title,
		Description:        description,
		ScheduledStartTime: start,
		ScheduledEndTime:   &end,
		EntityType:         discord.ScheduledEventEntityTypeExternal,
		PrivacyLevel:       discord.ScheduledEventPrivacyLevelGuildOnly,
		Image:              image,
		EntityMetaData:     &discord.EntityMetaData{Location: info.URL},
	}

	event, err := client.Rest.CreateGuildScheduledEvent(guildID, eventCreate)
	var restErr *rest.Error
	if image != nil && errors.As(err, &restErr) && restErr.Message == "Unsupported image type given" {
		log.Warn("Discord rejected the CTF logo, creating event without image", "logo", info.Logo)
		eventCreate.Image = nil
		event, err = client.Rest.CreateGuildScheduledEvent(guildID, eventCreate)
	}
	if err != nil {
		return nil, fmt.Errorf("create scheduled event: %w", err)
	}
	return event, nil
}

// Remove deletes everything the CTF owns on Discord, then its record.
func (s *Service) Remove(client *bot.Client, c store.CTF) error {
	// A channel already gone, or never created by a failed /create, must not make the CTF unremovable.
	if c.TextChannelID != 0 {
		err := client.Rest.DeleteChannel(c.TextChannelID)
		if err != nil && !isRESTCode(err, rest.JSONErrorCodeUnknownChannel) {
			return fmt.Errorf("delete channel: %w", err)
		}
	}

	if c.RoleID != 0 {
		if err := client.Rest.DeleteRole(c.ServerID, c.RoleID); err != nil {
			log.Error("failed to delete CTF role", "err", err, "role_id", c.RoleID)
		}
	}
	if c.EventID != 0 {
		if err := client.Rest.DeleteGuildScheduledEvent(c.ServerID, c.EventID); err != nil {
			log.Error("failed to delete CTF event", "err", err, "event_id", c.EventID)
		}
	}
	if c.MsgID != 0 {
		server, err := s.store.Server(c.ServerID)
		if err != nil {
			return err
		}
		if err := client.Rest.DeleteMessage(server.FeedChannelID, c.MsgID); err != nil {
			log.Error("failed to delete feed message", "err", err, "msg_id", c.MsgID)
		}
	}

	return s.store.DeleteCTF(c.ID)
}

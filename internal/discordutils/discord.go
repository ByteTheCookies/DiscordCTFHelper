package discordutils

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"golang.org/x/exp/rand"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/ctftime"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/database"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/models"
)

// Length limits enforced by Discord, in characters.
const (
	MaxEventDescLength   = 1000
	MaxOptionLabelLength = 100
	MaxOptionDescLength  = 100
)

// Truncate shortens s to at most maxLen characters without splitting one.
func Truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen-3]) + "..."
}

// PermissionDenied is shown to users who lack the manager role.
const PermissionDenied = "You don't have the required role to run this command. ❌"

// IsManager reports whether member holds the manager role configured for the guild.
func IsManager(guildID snowflake.ID, member *discord.ResolvedMember) (bool, error) {
	if member == nil {
		return false, nil
	}
	var server models.Server
	if err := server.GetByID(database.GetInstance().Connection(), guildID); err != nil {
		return false, err
	}
	return server.RoleManagerID != 0 && slices.Contains(member.RoleIDs, server.RoleManagerID), nil
}

// CheckPermission reports whether the user may run a manager command. When the
// answer is false the user has already been told, so the caller must just stop.
// The interaction must already be deferred.
func CheckPermission(e *handler.CommandEvent) (bool, error) {
	if e.GuildID() == nil {
		return false, nil
	}
	allowed, err := IsManager(*e.GuildID(), e.Member())
	if err != nil {
		log.Error("failed to check permissions", "error", err)
		_, _ = e.CreateFollowupMessage(discord.MessageCreate{
			Content: "failed to verify your permissions. ❌",
			Flags:   discord.MessageFlagEphemeral,
		})
		return false, err
	}
	if allowed {
		return true, nil
	}
	_, err = e.CreateFollowupMessage(discord.MessageCreate{
		Content: PermissionDenied,
		Flags:   discord.MessageFlagEphemeral,
	})
	return false, err
}

// IsCTFActive reports whether the CTF's scheduled event is running right now.
func IsCTFActive(client *bot.Client, ctf models.CTF) (bool, error) {
	if ctf.EventID == 0 {
		return false, nil
	}
	event, err := client.Rest.GetGuildScheduledEvent(ctf.ServerID, ctf.EventID, false)
	var restErr *rest.Error
	if errors.As(err, &restErr) && restErr.Code == rest.JSONErrorCodeUnknownGuildScheduledEvent {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return event.Status == discord.ScheduledEventStatusActive, nil
}

func CreateEvents(client *bot.Client, guildID *snowflake.ID, info ctftime.Event, eventDescription string, startTime time.Time, endTime time.Time) (discord.GuildScheduledEvent, error) {
	var image *discord.Icon
	if info.Logo != "" {
		imageLogo, imageType, err := ctftime.GetLogo(info.Logo)
		if err != nil {
			// A missing or unreadable logo must not cost the whole event.
			log.Warn("failed to fetch the CTF logo, creating event without image", "logo", info.Logo, "err", err)
		} else {
			log.Debug("Creating event with logo", "imageType", imageType)
			image = &discord.Icon{
				Data: imageLogo,
				Type: discord.IconType("image/" + imageType),
			}
		}
	}

	eventCreate := discord.GuildScheduledEventCreate{
		Name:               info.Title,
		Description:        eventDescription,
		ScheduledStartTime: startTime,
		ScheduledEndTime:   &endTime,
		EntityType:         discord.ScheduledEventEntityTypeExternal,
		PrivacyLevel:       discord.ScheduledEventPrivacyLevelGuildOnly,
		Image:              image,
		EntityMetaData: &discord.EntityMetaData{
			Location: info.URL,
		},
	}

	scheduledEvent, err := client.Rest.CreateGuildScheduledEvent(*guildID, eventCreate)
	var restErr *rest.Error
	if image != nil && errors.As(err, &restErr) && restErr.Message == "Unsupported image type given" {
		log.Warn("Discord rejected the CTF logo, creating event without image", "logo", info.Logo)
		eventCreate.Image = nil
		scheduledEvent, err = client.Rest.CreateGuildScheduledEvent(*guildID, eventCreate)
	}
	if err != nil {
		return discord.GuildScheduledEvent{}, err
	}

	return *scheduledEvent, nil
}

func CreateChannel(client *bot.Client, guildID *snowflake.ID, channelName string, categoryID snowflake.ID, roleID snowflake.ID, managerID snowflake.ID) (*discord.GuildChannel, error) {
	log.Debug("CreateChannel called with",
		"channelName", channelName,
		"categoryID", categoryID,
		"roleID", roleID,
		"managerID", managerID,
	)

	log.Debug("Fetching category channel", "categoryID", categoryID)
	category, err := client.Rest.GetChannel(categoryID)
	if err != nil {
		log.Error("failed to fetch category channel", "err", err)
		return nil, errors.New("invalid category")
	}
	if category.Type() != discord.ChannelTypeGuildCategory {
		log.Error("Channel is not a category", "type", category.Type())
		return nil, errors.New("invalid category")
	}

	var everyoneID *snowflake.ID
	roles, err := client.Rest.GetRoles(*guildID)
	if err != nil {
		log.Debug("failed to fetch roles", "err", err)
		return nil, err
	}
	for _, role := range roles {
		if role.Name == "@everyone" {
			log.Debug("Found @everyone role", "roleID", role.ID)
			everyoneID = &role.ID
			break
		}
	}

	if everyoneID == nil {
		log.Debug("Could not find @everyone role in guild", "guildID", *guildID)
		return nil, errors.New("could not find @everyone role in guild")
	}

	log.Debug("Creating guild text channel",
		"channelName", channelName,
		"categoryID", categoryID,
	)
	channel, err := client.Rest.CreateGuildChannel(*guildID, discord.GuildTextChannelCreate{
		Name:     channelName,
		ParentID: categoryID,
		PermissionOverwrites: []discord.PermissionOverwrite{
			discord.RolePermissionOverwrite{
				RoleID: *everyoneID,
				Deny:   discord.PermissionViewChannel,
			},
			discord.RolePermissionOverwrite{
				RoleID: roleID,
				Allow:  discord.PermissionViewChannel | discord.PermissionSendMessages,
			},
			discord.RolePermissionOverwrite{
				RoleID: managerID,
				Deny:   discord.PermissionViewChannel,
			},
		},
	})
	if err != nil {
		log.Debug("failed to create guild channel", "err", err)
		return nil, err
	}

	log.Debug("Successfully created channel",
		"channelName", channel.Name(),
		"channelID", channel.ID(),
	)
	return &channel, nil
}

func CreateEmbed(client *bot.Client, data ctftime.Event, startTime time.Time, endTime time.Time, channel discord.Channel) (*discord.Message, error) {
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

`, Truncate(data.Description, 2500), int(startTime.Unix()), int(endTime.Unix()), data.URL, data.Format, data.Location, data.Weight, Truncate(data.Prizes, 1000))

	t := time.Now()
	embed := discord.Embed{
		Title:       data.Title,
		Description: description,
		URL:         data.URL,
		Timestamp:   &t,
		Color:       0xBEBEFE,
		Thumbnail: &discord.EmbedResource{
			URL: data.Logo,
		},
		Footer: &discord.EmbedFooter{
			Text: "Add a reaction to get the ctf role (only if you want to participate). 🙃",
		},
	}

	message, err := client.Rest.CreateMessage(channel.ID(), discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
	if err != nil {
		return nil, err
	}

	// Add reaction
	err = client.Rest.AddReaction(channel.ID(), message.ID, "✅")
	if err != nil {
		return nil, err
	}

	return message, nil
}

func CreateRole(client *bot.Client, guildID *snowflake.ID, name string) (*discord.Role, error) {
	if guildID == nil {
		return nil, nil
	}

	color := rand.Intn(0xFFFFFF)
	for color == 0xBEBEFE {
		color = rand.Intn(0xFFFFFF)
	}

	role, err := client.Rest.CreateRole(*guildID, discord.RoleCreate{
		Name:        name,
		Color:       color,
		Mentionable: true,
		Hoist:       true,
	})
	return role, err
}

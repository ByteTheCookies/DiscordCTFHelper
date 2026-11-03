package discordutils

import (
	"ctftime"
	"database"
	"errors"
	"fmt"
	"models"
	"slices"
	"time"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"golang.org/x/exp/rand"
)

const MaxDescLength = 4096

func CheckPermission(e *handler.CommandEvent) error {
	db := database.GetInstance().Connection()
	sendEphemeral := func(content string) error {
		_, err := e.CreateFollowupMessage(discord.MessageCreate{
			Content: content,
			Flags:   discord.MessageFlagEphemeral,
		})
		return err
	}

	guildID := e.GuildID()
	if guildID == nil {
		return sendEphemeral("Guild not found. ❌")
	}

	guild, err := e.Client().Rest.GetGuild(*guildID, false)
	if err != nil {
		return err
	}

	var server models.Server
	err = server.GetByID(db, *guildID)
	if err != nil {
		return err
	}
	if server == (models.Server{}) {
		return sendEphemeral("Server configuration not found. ❌")
	}

	var roleManager *discord.Role
	for i := range guild.Roles {
		if guild.Roles[i].ID == server.RoleManagerID {
			roleManager = &guild.Roles[i]
			break
		}
	}
	member := e.Member()

	hasRole := false
	if member != nil && roleManager != nil {
		hasRole = slices.Contains(member.RoleIDs, roleManager.ID)
	}

	if member == nil || roleManager == nil || !hasRole {
		return sendEphemeral("You don't have the required role to run this command. ❌")
	}

	return nil
}

func CreateEvents(client *bot.Client, guildID *snowflake.ID, info ctftime.Event, eventDescription string, startTime time.Time, endTime time.Time) (discord.GuildScheduledEvent, error) {
	var image *discord.Icon
	if info.Logo != "" {
		imageLogo, imageType, err := ctftime.GetLogo(info.Logo)
		if err != nil {
			return discord.GuildScheduledEvent{}, err
		}
		log.Debug("Creating event with logo", "imageType", imageType)

		image = &discord.Icon{
			Data: imageLogo,
			Type: discord.IconType(imageType),
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
		log.Debug("failed to fetch category channel", "err", err)
	}
	if err != nil || category.Type() != discord.ChannelTypeGuildCategory {
		log.Debug("Category channel is invalid or not a category", "type", category.Type())
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

`, data.Description, int(startTime.Unix()), int(endTime.Unix()), data.URL, data.Format, data.Location, data.Weight, data.Prizes)

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

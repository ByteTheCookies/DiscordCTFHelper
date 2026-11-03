package commands

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/store"
)

var initCommand = discord.SlashCommandCreate{
	Name:        "init",
	Description: "Initialize the CTF bot in the discord server.",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionChannel{
			Name:         "category_active",
			Description:  "The name of the category for the next or current ctf",
			Required:     true,
			ChannelTypes: []discord.ChannelType{discord.ChannelTypeGuildCategory},
		},
		discord.ApplicationCommandOptionChannel{
			Name:         "category_archived",
			Description:  "The name of the category for the archived ctf",
			Required:     true,
			ChannelTypes: []discord.ChannelType{discord.ChannelTypeGuildCategory},
		},
		discord.ApplicationCommandOptionRole{
			Name:        "role_manager",
			Description: "The only role that can run the create_ctf command",
			Required:    true,
		},
		discord.ApplicationCommandOptionChannel{
			Name:         "feed_channel",
			Description:  "The channel feed for publish the ctf",
			Required:     true,
			ChannelTypes: []discord.ChannelType{discord.ChannelTypeGuildText},
		},
		discord.ApplicationCommandOptionInt{
			Name:        "team_id",
			Description: "The id of the team",
			Required:    true,
		},
		discord.ApplicationCommandOptionRole{
			Name:        "role_team_id",
			Description: "The role id of the team for tagging purposes",
			Required:    true,
		},
	},
}

func (h *Handlers) init(e *handler.CommandEvent) error {
	if !e.Member().Permissions.Has(discord.PermissionAdministrator) {
		return reply(e, "You need to be the admin of the server to run this command. ❌")
	}

	options := e.SlashCommandInteractionData()
	err := h.store.ReplaceServer(store.Server{
		ID:                *e.GuildID(),
		ActiveCategoryID:  options.Channel("category_active").ID,
		ArchiveCategoryID: options.Channel("category_archived").ID,
		RoleManagerID:     options.Role("role_manager").ID,
		FeedChannelID:     options.Channel("feed_channel").ID,
		TeamID:            int64(options.Int("team_id")),
		RoleTeamID:        options.Role("role_team_id").ID,
	})
	if err != nil {
		return err
	}
	return reply(e, "Successfully configured the bot! ✅")
}

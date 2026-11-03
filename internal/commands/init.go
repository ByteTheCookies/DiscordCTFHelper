package commands

import (
	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/database"
	"github.com/ByteTheCookies/DiscordCTFHelper/internal/models"
)

var cinit = discord.SlashCommandCreate{
	Name:        "init",
	Description: "Initialize the CTF bot in the discord server.",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionChannel{
			Name:        "category_active",
			Description: "The name of the category for the next or current ctf",
			Required:    true,
			ChannelTypes: []discord.ChannelType{
				discord.ChannelTypeGuildCategory,
			},
		},
		discord.ApplicationCommandOptionChannel{
			Name:        "category_archived",
			Description: "The name of the category for the archived ctf",
			Required:    true,
			ChannelTypes: []discord.ChannelType{
				discord.ChannelTypeGuildCategory,
			},
		},
		discord.ApplicationCommandOptionRole{
			Name:        "role_manager",
			Description: "The only role that can run the create_ctf command",
			Required:    true,
		},
		discord.ApplicationCommandOptionChannel{
			Name:        "feed_channel",
			Description: "The channel feed for publish the ctf",
			Required:    true,
			ChannelTypes: []discord.ChannelType{
				discord.ChannelTypeGuildText,
			},
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

func InitHandler() handler.CommandHandler {
	log.Info("Setting up InitHandler")
	return func(e *handler.CommandEvent) error {
		db := database.GetInstance().Connection()
		log.Debug("InitHandler called", "guild_id", e.GuildID())

		if err := e.DeferCreateMessage(true); err != nil {
			log.Error("failed to defer create message", "error", err)
			return err
		}

		if e.Member() == nil || !e.Member().Permissions.Has(discord.PermissionAdministrator) {
			log.Warn("User tried to run /init without admin permissions", "user_id", e.User().ID, "guild_id", e.GuildID())
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "You need to be the admin of the server to run this command. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		if e.GuildID() == nil {
			log.Warn("Init command used outside of a guild", "user_id", e.User().ID)
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "This command can only be used inside a guild. ❌",
				Flags:   discord.MessageFlagEphemeral,
			})
			return err
		}

		options := e.SlashCommandInteractionData()

		categoryActive := options.Channel("category_active")
		archiveCategory := options.Channel("category_archived")
		roleManager := options.Role("role_manager")
		feedChannel := options.Channel("feed_channel")
		teamID := options.Int("team_id")
		roleTeam := options.Role("role_team_id")

		log.Debug("Parsed options",
			"category_active", categoryActive.ID,
			"category_archived", archiveCategory.ID,
			"role_manager", roleManager.ID,
			"feed_channel", feedChannel.ID,
			"team_id", teamID,
			"role_team_id", roleTeam,
			"guild_id", e.GuildID(),
		)

		var server models.Server
		err := server.GetByID(db, *e.GuildID())
		if err != nil {
			log.Error("failed to get server by ID", "guild_id", e.GuildID(), "error", err)
			return err
		}
		if server != (models.Server{}) {
			log.Info("Server already exists, deleting old config", "guild_id", e.GuildID())
			if err := server.Delete(db); err != nil {
				log.Error("failed to delete existing server config", "guild_id", e.GuildID(), "error", err)
				return err
			}
		}

		log.Info("Adding new server config", "guild_id", e.GuildID())
		server = models.Server{
			ID:                *e.GuildID(),
			ActiveCategoryID:  categoryActive.ID,
			ArchiveCategoryID: archiveCategory.ID,
			RoleManagerID:     roleManager.ID,
			FeedChannelID:     feedChannel.ID,
			TeamID:            int64(teamID),
			RoleTeamID:        roleTeam.ID,
		}
		if err := server.Add(db); err != nil {
			log.Error("failed to add server config", "guild_id", e.GuildID(), "error", err)
			return err
		}

		log.Info("Successfully configured the bot", "guild_id", e.GuildID())
		_, err = e.CreateFollowupMessage(discord.MessageCreate{
			Content: "Successfully configured the bot! ✅",
			Flags:   discord.MessageFlagEphemeral,
		})
		return err
	}
}

package models

import (
	"time"

	"github.com/disgoorg/snowflake/v2"
	"gorm.io/gorm"
)

// Server represents a Discord server configuration
type Server struct {
	gorm.Model
	ID                snowflake.ID `gorm:"primaryKey"`
	ActiveCategoryID  snowflake.ID
	ArchiveCategoryID snowflake.ID
	RoleManagerID     snowflake.ID
	FeedChannelID     snowflake.ID
	TeamID            int64
	RoleTeamID        snowflake.ID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (server *Server) Add(db *gorm.DB) error {
	return db.Create(server).Error
}

func (server *Server) GetByID(db *gorm.DB, serverID snowflake.ID) error {
	result := db.First(&server, Server{ID: serverID})
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil
		}
		return result.Error
	}
	return nil
}

func (server *Server) EditCategory(db *gorm.DB, activeCategoryID, archiveCategoryID snowflake.ID) error {
	return db.Model(&Server{}).
		Where(Server{ID: server.ID}).
		Updates(Server{
			ActiveCategoryID:  activeCategoryID,
			ArchiveCategoryID: archiveCategoryID,
		}).Error
}

func (server *Server) Delete(db *gorm.DB) error {
	return db.Unscoped().Delete(&Server{}, server).Error
}

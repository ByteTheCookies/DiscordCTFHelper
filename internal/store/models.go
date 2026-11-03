package store

import (
	"slices"
	"time"

	"github.com/disgoorg/snowflake/v2"
	"gorm.io/gorm"
)

// Server is the bot configuration of a Discord server.
type Server struct {
	ID                snowflake.ID `gorm:"primaryKey"`
	ActiveCategoryID  snowflake.ID
	ArchiveCategoryID snowflake.ID
	RoleManagerID     snowflake.ID
	FeedChannelID     snowflake.ID
	TeamID            int64
	RoleTeamID        snowflake.ID
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         gorm.DeletedAt `gorm:"index"`
}

// IsManager reports whether roles include the server's manager role.
func (s Server) IsManager(roles []snowflake.ID) bool {
	return s.RoleManagerID != 0 && slices.Contains(roles, s.RoleManagerID)
}

// CTF is a CTF event published on a Discord server.
type CTF struct {
	ID            int64        `gorm:"primaryKey"`
	ServerID      snowflake.ID `gorm:"not null"`
	Name          string       `gorm:"not null"`
	Description   string
	TextChannelID snowflake.ID
	EventID       snowflake.ID
	RoleID        snowflake.ID
	MsgID         snowflake.ID
	CTFTimeID     int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

// Report holds the results of a team in a CTF. Place and Score are -1 until known.
type Report struct {
	ID        int64 `gorm:"primaryKey"`
	CTFID     int64 `gorm:"not null"`
	Place     int
	Solves    int
	Score     int
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// Creds holds the shared credentials of a CTF.
type Creds struct {
	ID        int64 `gorm:"primaryKey"`
	Username  string
	Password  string
	Personal  bool
	CTFID     int64 `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

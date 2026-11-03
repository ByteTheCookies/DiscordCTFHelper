package models

import (
	"time"

	"github.com/disgoorg/snowflake/v2"
	"gorm.io/gorm"
)

// CTF represents a CTF event
type CTF struct {
	gorm.Model
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
}

// Add adds the CTF to the database.
func (ctf *CTF) Add(db *gorm.DB) error {
	return db.Create(ctf).Error
}

// GetByID retrieves a CTF by its ID.
func (ctf *CTF) GetByID(db *gorm.DB, id int64) error {
	result := db.First(ctf, CTF{ID: id})
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil
		}
		return result.Error
	}
	return nil
}

// GetByName retrieves a CTF by name and server ID (LIKE match).
func (ctf *CTF) GetByName(db *gorm.DB, name string, serverID snowflake.ID) error {
	result := db.Where("name LIKE ? AND server_id = ?", "%"+name+"%", serverID).First(ctf)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil
		}
		return result.Error
	}
	return nil
}

// GetByTimeID retrieves a CTF by its CTFTimeID.
func (ctf *CTF) GetByTimeID(db *gorm.DB, ctftimeID int64) error {
	result := db.Where(CTF{CTFTimeID: ctftimeID}).First(ctf)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil
		}
		return result.Error
	}
	return nil
}

// GetByMessageID retrieves a CTF by message ID and server ID.
func (ctf *CTF) GetByMessageID(db *gorm.DB, messageID, serverID snowflake.ID) error {
	result := db.Where(CTF{MsgID: messageID, ServerID: serverID}).First(ctf)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil
		}
		return result.Error
	}
	return nil
}

// GetByChannelID retrieves a CTF by text channel ID and server ID.
func (ctf *CTF) GetByChannelID(db *gorm.DB, channelID, serverID snowflake.ID) error {
	result := db.Where(CTF{TextChannelID: channelID, ServerID: serverID}).First(ctf)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil
		}
		return result.Error
	}
	return nil
}

// Delete deletes the CTF from the database by its ID, along with its credentials and report.
func (ctf *CTF) Delete(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		creds := Creds{CTFID: ctf.ID}
		if err := creds.Delete(tx); err != nil {
			return err
		}
		if err := (&Report{}).Delete(tx, ctf.ID); err != nil {
			return err
		}
		return tx.Delete(&CTF{}, ctf.ID).Error
	})
}

// IsPresent checks if a CTF is present in the database by name and server ID.
func (ctf *CTF) IsPresent(db *gorm.DB) (bool, error) {
	var count int64
	err := db.Model(&CTF{}).Where(CTF{Name: ctf.Name, ServerID: ctf.ServerID}).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetList retrieves a list of all CTFs for a specific server.
func (CTF) GetList(db *gorm.DB, serverID snowflake.ID) ([]CTF, error) {
	var ctfs []CTF
	err := db.Where(CTF{ServerID: serverID}).Find(&ctfs).Error
	if err != nil {
		return nil, err
	}
	return ctfs, nil
}

func (ctf CTF) Update(db *gorm.DB) error {
	return db.Save(ctf).Error
}

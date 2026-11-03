package models

import (
	"time"

	"gorm.io/gorm"
)

// Creds represents credentials for a CTF
type Creds struct {
	gorm.Model
	ID        int64 `gorm:"primaryKey"`
	Username  string
	Password  string
	Personal  bool
	CTFID     int64 `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Add adds credentials to the database.
func (creds *Creds) Add(db *gorm.DB) error {
	creds.UpdatedAt = time.Now()
	return db.Create(creds).Error
}

// GetByCTFID retrieves credentials from the database for a specific CTF.
func (creds *Creds) GetByCTFID(db *gorm.DB, ctfID int64) error {
	result := db.Where(Creds{CTFID: ctfID}).First(creds)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil
		}
		return result.Error
	}
	return nil
}

// Update updates credentials in the database. If they do not exist, it creates them.
func (creds *Creds) Update(db *gorm.DB, ctfID int64) error {
	result := db.Model(&Creds{}).
		Where(Creds{CTFID: ctfID}).
		Updates(Creds{
			Username: creds.Username,
			Password: creds.Password,
			Personal: creds.Personal,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		creds.CTFID = ctfID
		return db.Create(creds).Error
	}

	return nil
}

// Delete permanently erases credentials by CTFID: passwords must not survive in a soft-deleted row.
func (creds *Creds) Delete(db *gorm.DB) error {
	return db.Unscoped().Delete(&Creds{}, Creds{CTFID: creds.CTFID}).Error
}

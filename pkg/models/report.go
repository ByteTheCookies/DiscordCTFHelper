package models

import (
	"time"

	"gorm.io/gorm"
)

// Report represents a CTF report with results
type Report struct {
	gorm.Model
	ID        int64 `gorm:"primaryKey"`
	CTFID     int64 `gorm:"not null"`
	Place     int
	Solves    int
	Score     int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Add adds a report to the database.
func (report *Report) Add(db *gorm.DB) error {
	report.UpdatedAt = time.Now()
	return db.Create(report).Error
}

// GetByCTFID retrieves a report from the database for a specific CTF.
func (report *Report) GetByCTFID(db *gorm.DB, ctfID int64) error {
	result := db.Where(Report{CTFID: ctfID}).First(report)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil
		}
		return result.Error
	}
	return nil
}

// Update updates a report in the database. If it does not exist, it creates it.
func (report *Report) Update(db *gorm.DB) error {
	// Select makes GORM write zero values too, or solves could never go back to 0.
	result := db.Model(&Report{}).
		Where(Report{CTFID: report.CTFID}).
		Select("Place", "Solves", "Score").
		Updates(Report{
			Place:  report.Place,
			Solves: report.Solves,
			Score:  report.Score,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return db.Create(report).Error
	}

	return nil
}

// Delete deletes a report from the database by CTFID.
func (*Report) Delete(db *gorm.DB, ctfID int64) error {
	return db.Delete(&Report{}, Report{CTFID: ctfID}).Error
}

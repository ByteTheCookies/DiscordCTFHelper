// Package store persists the bot's data in SQLite.
package store

import (
	"errors"
	"fmt"
	stdlog "log"
	"os"
	"time"

	"github.com/disgoorg/snowflake/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	// ErrNotFound is returned when the requested record does not exist.
	ErrNotFound = errors.New("record not found")
	// ErrExists is returned when the record to add is already there.
	ErrExists = errors.New("record already exists")
)

// Store gives access to the database.
type Store struct {
	db *gorm.DB
}

// Open opens the database at path and migrates its schema.
func Open(path string) (*Store, error) {
	// Other processes, such as backups, may hold the file, so wait for them. Taking
	// the write lock up front matters for transactions that read before writing: SQLite
	// fails those at once, without waiting, if another writer got in between.
	db, err := gorm.Open(sqlite.Open(path+"?_busy_timeout=5000&_txlock=immediate"), &gorm.Config{
		Logger: logger.New(stdlog.New(os.Stderr, "", stdlog.LstdFlags), logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	conn, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get database handle: %w", err)
	}
	// ponytail: SQLite has one writer anyway, and queueing here was 2.5x faster under
	// load than a pool polling for the lock. Reads wait too: move to WAL with a read
	// pool if that ever shows up. Inside a transaction use only tx, or it deadlocks.
	conn.SetMaxOpenConns(1)

	if err := db.AutoMigrate(&Server{}, &CTF{}, &Report{}, &Creds{}); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	conn, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("get database handle: %w", err)
	}
	return conn.Close()
}

// first returns the first T matching cond. A map is used for cond because GORM
// silently drops the zero fields of a struct, turning a lookup by 0 into "any row".
func first[T any](db *gorm.DB, cond map[string]any) (T, error) {
	var v T
	err := db.Where(cond).First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, fmt.Errorf("query %T: %w", v, err)
	}
	return v, nil
}

// Server returns the configuration of a Discord server.
func (s *Store) Server(id snowflake.ID) (Server, error) {
	return first[Server](s.db, map[string]any{"id": id})
}

// ReplaceServer stores server in place of any previous configuration.
func (s *Store) ReplaceServer(server Server) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("id = ?", server.ID).Delete(&Server{}).Error; err != nil {
			return fmt.Errorf("delete old server: %w", err)
		}
		if err := tx.Create(&server).Error; err != nil {
			return fmt.Errorf("create server: %w", err)
		}
		return nil
	})
}

// CTFByID returns a CTF by its ID.
func (s *Store) CTFByID(id int64) (CTF, error) {
	return first[CTF](s.db, map[string]any{"id": id})
}

// CTFByChannel returns the CTF that owns a text channel.
func (s *Store) CTFByChannel(channelID, serverID snowflake.ID) (CTF, error) {
	return first[CTF](s.db, map[string]any{"text_channel_id": channelID, "server_id": serverID})
}

// CTFByMessage returns the CTF announced by a feed message.
func (s *Store) CTFByMessage(messageID, serverID snowflake.ID) (CTF, error) {
	return first[CTF](s.db, map[string]any{"msg_id": messageID, "server_id": serverID})
}

// CTFByEvent returns the CTF that owns a scheduled event.
func (s *Store) CTFByEvent(eventID, serverID snowflake.ID) (CTF, error) {
	return first[CTF](s.db, map[string]any{"event_id": eventID, "server_id": serverID})
}

// ListCTFs returns all the CTFs of a server.
func (s *Store) ListCTFs(serverID snowflake.ID) ([]CTF, error) {
	var ctfs []CTF
	if err := s.db.Where("server_id = ?", serverID).Find(&ctfs).Error; err != nil {
		return nil, fmt.Errorf("list CTFs: %w", err)
	}
	return ctfs, nil
}

// AddCTF creates ctf and sets its ID, or returns ErrExists if the server already
// has a CTF with this name. Concurrent calls for the same name can't both succeed.
func (s *Store) AddCTF(ctf *CTF) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&CTF{}).Where("name = ? AND server_id = ?", ctf.Name, ctf.ServerID).Count(&count).Error; err != nil {
			return fmt.Errorf("count CTFs: %w", err)
		}
		if count > 0 {
			return ErrExists
		}
		if err := tx.Create(ctf).Error; err != nil {
			return fmt.Errorf("create CTF: %w", err)
		}
		return nil
	})
}

// SaveCTF writes every field of ctf.
func (s *Store) SaveCTF(ctf *CTF) error {
	if err := s.db.Save(ctf).Error; err != nil {
		return fmt.Errorf("save CTF: %w", err)
	}
	return nil
}

// DeleteCTF deletes a CTF along with its credentials and report.
func (s *Store) DeleteCTF(id int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteCreds(tx, id); err != nil {
			return err
		}
		if err := tx.Where("ctf_id = ?", id).Delete(&Report{}).Error; err != nil {
			return fmt.Errorf("delete report: %w", err)
		}
		if err := tx.Where("id = ?", id).Delete(&CTF{}).Error; err != nil {
			return fmt.Errorf("delete CTF: %w", err)
		}
		return nil
	})
}

// Report returns the report of a CTF.
func (s *Store) Report(ctfID int64) (Report, error) {
	return first[Report](s.db, map[string]any{"ctf_id": ctfID})
}

// AddSolve counts a solve in the report of a CTF, creating the report if needed,
// and returns the new total.
func (s *Store) AddSolve(ctfID int64) (int, error) {
	var report Report
	err := s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Report{}).Where("ctf_id = ?", ctfID).Update("solves", gorm.Expr("solves + 1"))
		if result.Error != nil {
			return fmt.Errorf("update solves: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			if err := tx.Create(&Report{CTFID: ctfID, Place: -1, Score: -1, Solves: 1}).Error; err != nil {
				return fmt.Errorf("create report: %w", err)
			}
		}
		var err error
		report, err = first[Report](tx, map[string]any{"ctf_id": ctfID})
		return err
	})
	return report.Solves, err
}

// RemoveSolve takes back a solve from the report of a CTF, if it has any.
func (s *Store) RemoveSolve(ctfID int64) error {
	err := s.db.Model(&Report{}).Where("ctf_id = ? AND solves > 0", ctfID).Update("solves", gorm.Expr("solves - 1")).Error
	if err != nil {
		return fmt.Errorf("update solves: %w", err)
	}
	return nil
}

// SetResults stores the standing of a CTF, creating the report if needed. Solves
// are only written when positive, to keep the count kept by AddSolve otherwise.
func (s *Store) SetResults(ctfID int64, place, score, solves int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		values := map[string]any{"place": place, "score": score}
		if solves > 0 {
			values["solves"] = solves
		}
		result := tx.Model(&Report{}).Where("ctf_id = ?", ctfID).Updates(values)
		if result.Error != nil {
			return fmt.Errorf("update report: %w", result.Error)
		}
		if result.RowsAffected > 0 {
			return nil
		}
		if err := tx.Create(&Report{CTFID: ctfID, Place: place, Score: score, Solves: solves}).Error; err != nil {
			return fmt.Errorf("create report: %w", err)
		}
		return nil
	})
}

// Creds returns the credentials of a CTF.
func (s *Store) Creds(ctfID int64) (Creds, error) {
	return first[Creds](s.db, map[string]any{"ctf_id": ctfID})
}

// AddCreds stores credentials, or returns ErrExists if the CTF already has some.
func (s *Store) AddCreds(creds Creds) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&Creds{}).Where("ctf_id = ?", creds.CTFID).Count(&count).Error; err != nil {
			return fmt.Errorf("count credentials: %w", err)
		}
		if count > 0 {
			return ErrExists
		}
		if err := tx.Create(&creds).Error; err != nil {
			return fmt.Errorf("create credentials: %w", err)
		}
		return nil
	})
}

// DeleteCreds erases the credentials of a CTF.
func (s *Store) DeleteCreds(ctfID int64) error {
	return deleteCreds(s.db, ctfID)
}

// deleteCreds hard-deletes: passwords must not survive in a soft-deleted row.
func deleteCreds(db *gorm.DB, ctfID int64) error {
	if err := db.Unscoped().Where("ctf_id = ?", ctfID).Delete(&Creds{}).Error; err != nil {
		return fmt.Errorf("delete credentials: %w", err)
	}
	return nil
}

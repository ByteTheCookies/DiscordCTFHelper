package store

import (
	"errors"
	"testing"

	"github.com/disgoorg/snowflake/v2"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func addCTF(t *testing.T, s *Store) CTF {
	t.Helper()
	ctf := CTF{Name: t.Name(), ServerID: 1}
	if err := s.AddCTF(&ctf); err != nil {
		t.Fatal(err)
	}
	return ctf
}

func countCreds(t *testing.T, s *Store, ctfID int64) int64 {
	t.Helper()
	var n int64
	if err := s.db.Unscoped().Model(&Creds{}).Where("ctf_id = ?", ctfID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLookupsReportNotFound(t *testing.T) {
	s := open(t)
	row := CTF{Name: t.Name(), ServerID: 1, TextChannelID: 5}
	if err := s.AddCTF(&row); err != nil {
		t.Fatal(err)
	}
	// 0 must look for 0, not match any row: the trap of GORM's struct conditions.
	if _, err := s.CTFByID(0); !errors.Is(err, ErrNotFound) {
		t.Errorf("CTFByID(0) err = %v", err)
	}
	if _, err := s.CTFByChannel(0, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("CTFByChannel(0) err = %v", err)
	}
	if _, err := s.Server(1); !errors.Is(err, ErrNotFound) {
		t.Errorf("Server err = %v", err)
	}
	if _, err := s.Report(1); !errors.Is(err, ErrNotFound) {
		t.Errorf("Report err = %v", err)
	}
	if _, err := s.Creds(1); !errors.Is(err, ErrNotFound) {
		t.Errorf("Creds err = %v", err)
	}
}

func TestSaveReportCreatesThenWritesZeros(t *testing.T) {
	s := open(t)
	if err := s.SaveReport(Report{CTFID: 7, Place: -1, Score: -1, Solves: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveReport(Report{CTFID: 7, Place: 0, Score: 0, Solves: 0}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Report(7)
	if err != nil || got.Place != 0 || got.Score != 0 || got.Solves != 0 {
		t.Fatalf("report = %+v, err %v", got, err)
	}
}

func TestSaveCTFKeepsChanges(t *testing.T) {
	s := open(t)
	ctf := addCTF(t, s)
	ctf.RoleID = 55
	if err := s.SaveCTF(&ctf); err != nil {
		t.Fatal(err)
	}
	got, err := s.CTFByID(ctf.ID)
	if err != nil || got.RoleID != 55 {
		t.Fatalf("ctf = %+v, err %v", got, err)
	}
}

func TestReplaceServerAffectsOnlyThatServer(t *testing.T) {
	s := open(t)
	for _, srv := range []Server{{ID: 1, TeamID: 1}, {ID: 2, TeamID: 2}, {ID: 1, TeamID: 3}} {
		if err := s.ReplaceServer(srv); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := s.Server(1); err != nil || got.TeamID != 3 {
		t.Fatalf("server 1 = %+v, err %v", got, err)
	}
	if got, err := s.Server(2); err != nil || got.TeamID != 2 {
		t.Fatalf("server 2 = %+v, err %v", got, err)
	}
}

func TestDeleteCredsErasesThePassword(t *testing.T) {
	s := open(t)
	if err := s.AddCreds(Creds{Username: "u", Password: "secret", CTFID: 7}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCreds(7); err != nil {
		t.Fatal(err)
	}
	if n := countCreds(t, s, 7); n != 0 {
		t.Fatalf("%d credential rows survive, including soft-deleted ones", n)
	}
}

func TestDeleteCTFTakesCredsAndReportWithIt(t *testing.T) {
	s := open(t)
	ctf, other := addCTF(t, s), addCTF(t, s)
	for _, id := range []int64{ctf.ID, other.ID} {
		err := errors.Join(
			s.AddCreds(Creds{Username: "u", Password: "secret", CTFID: id}),
			s.SaveReport(Report{CTFID: id, Solves: 1}),
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := s.DeleteCTF(ctf.ID); err != nil {
		t.Fatal(err)
	}

	if n := countCreds(t, s, ctf.ID); n != 0 {
		t.Fatalf("%d credential rows survive the CTF", n)
	}
	if _, err := s.Report(ctf.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("report survives the CTF: %v", err)
	}
	if _, err := s.CTFByID(ctf.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CTF survives: %v", err)
	}
	if countCreds(t, s, other.ID) != 1 {
		t.Fatal("credentials of another CTF were deleted")
	}
	if _, err := s.Report(other.ID); err != nil {
		t.Fatalf("report of another CTF was deleted: %v", err)
	}
}

func TestIsManager(t *testing.T) {
	server := Server{RoleManagerID: 100}
	if !server.IsManager([]snowflake.ID{5, 100}) {
		t.Error("holder of the manager role rejected")
	}
	if server.IsManager([]snowflake.ID{5}) {
		t.Error("member without the manager role accepted")
	}
	if (Server{}).IsManager([]snowflake.ID{0}) {
		t.Error("server without a manager role accepted a member")
	}
}

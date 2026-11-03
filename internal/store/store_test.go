package store

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
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

func addCTF(t *testing.T, s *Store, name string) CTF {
	t.Helper()
	ctf := CTF{Name: name, ServerID: 1}
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

func TestSolvesCountUpAndDownToZero(t *testing.T) {
	s := open(t)
	for want := 1; want <= 2; want++ {
		if got, err := s.AddSolve(7); err != nil || got != want {
			t.Fatalf("AddSolve = %d, err %v, want %d", got, err, want)
		}
	}
	for range 3 { // One more than there are: it must stop at zero.
		if err := s.RemoveSolve(7); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Report(7)
	if err != nil || got.Solves != 0 || got.Place != -1 || got.Score != -1 {
		t.Fatalf("report = %+v, err %v", got, err)
	}
}

func TestSetResultsKeepsSolvesUnlessReported(t *testing.T) {
	s := open(t)
	if _, err := s.AddSolve(7); err != nil {
		t.Fatal(err)
	}
	if err := s.SetResults(7, 3, 100, 0); err != nil {
		t.Fatal(err)
	}
	got, err := s.Report(7)
	if err != nil || got.Place != 3 || got.Score != 100 || got.Solves != 1 {
		t.Fatalf("without solves: %+v, err %v", got, err)
	}
	if err := s.SetResults(7, 3, 100, 9); err != nil {
		t.Fatal(err)
	}
	if got, err = s.Report(7); err != nil || got.Solves != 9 {
		t.Fatalf("with solves: %+v, err %v", got, err)
	}
	if err := s.SetResults(8, 1, 2, 0); err != nil {
		t.Fatal(err)
	}
	if got, err = s.Report(8); err != nil || got.Place != 1 {
		t.Fatalf("new report: %+v, err %v", got, err)
	}
}

// concurrently runs f n times at once and returns how many calls succeeded.
func concurrently(t *testing.T, n int, f func() error) int {
	t.Helper()
	var (
		wg        sync.WaitGroup
		succeeded atomic.Int32
	)
	for range n {
		wg.Go(func() {
			err := f()
			if err == nil {
				succeeded.Add(1)
			} else if !errors.Is(err, ErrExists) {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
	wg.Wait()
	return int(succeeded.Load())
}

func TestConcurrentSolvesAreAllCounted(t *testing.T) {
	s := open(t)
	const n = 50
	concurrently(t, n, func() error {
		_, err := s.AddSolve(7)
		return err
	})
	got, err := s.Report(7)
	if err != nil || got.Solves != n {
		t.Fatalf("report = %+v, err %v, want %d solves", got, err, n)
	}
	var reports int64
	if err := s.db.Model(&Report{}).Where("ctf_id = 7").Count(&reports).Error; err != nil || reports != 1 {
		t.Fatalf("%d reports, err %v", reports, err)
	}
}

func TestConcurrentAddCTFCreatesOne(t *testing.T) {
	s := open(t)
	created := concurrently(t, 20, func() error { return s.AddCTF(&CTF{Name: "same", ServerID: 1}) })
	if created != 1 {
		t.Fatalf("%d CTFs created, want 1", created)
	}
}

func TestAddCTFAllowsSameNameElsewhere(t *testing.T) {
	s := open(t)
	addCTF(t, s, "same")
	if err := s.AddCTF(&CTF{Name: "same", ServerID: 2}); err != nil {
		t.Fatalf("another server: %v", err)
	}
	removed := addCTF(t, s, "removed")
	if err := s.DeleteCTF(removed.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.AddCTF(&CTF{Name: "removed", ServerID: 1}); err != nil {
		t.Fatalf("after removal: %v", err)
	}
}

func TestConcurrentAddCredsStoresOne(t *testing.T) {
	s := open(t)
	created := concurrently(t, 20, func() error { return s.AddCreds(Creds{CTFID: 7, Password: "p"}) })
	if created != 1 || countCreds(t, s, 7) != 1 {
		t.Fatalf("%d calls succeeded, %d rows", created, countCreds(t, s, 7))
	}
}

// Another process on the same file, such as a backup, must make us wait rather than fail.
func TestWritesWaitForOtherProcesses(t *testing.T) {
	path := t.TempDir() + "/test.db"
	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })

	const n = 40
	concurrently(t, n, func() error {
		_, err := a.AddSolve(7)
		return err
	})
	concurrently(t, n, func() error {
		_, err := b.AddSolve(8)
		return err
	})
	var wg sync.WaitGroup
	wg.Go(func() { concurrently(t, n, func() error { _, err := a.AddSolve(9); return err }) })
	wg.Go(func() { concurrently(t, n, func() error { _, err := b.AddSolve(9); return err }) })
	wg.Wait()

	got, err := a.Report(9)
	if err != nil || got.Solves != 2*n {
		t.Fatalf("report = %+v, err %v, want %d solves", got, err, 2*n)
	}

	// AddCTF reads before writing, the case SQLite fails at once unless the write
	// lock is taken up front.
	var names atomic.Int32
	for _, st := range []*Store{a, b} {
		wg.Go(func() {
			concurrently(t, 100, func() error {
				return st.AddCTF(&CTF{Name: strconv.Itoa(int(names.Add(1))), ServerID: 1})
			})
		})
	}
	wg.Wait()
	if ctfs, err := a.ListCTFs(1); err != nil || len(ctfs) != 200 {
		t.Fatalf("%d CTFs, err %v, want 200", len(ctfs), err)
	}
}

func TestSaveCTFKeepsChanges(t *testing.T) {
	s := open(t)
	ctf := addCTF(t, s, "a")
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
	ctf, other := addCTF(t, s, "a"), addCTF(t, s, "b")
	for _, id := range []int64{ctf.ID, other.ID} {
		err := errors.Join(
			s.AddCreds(Creds{Username: "u", Password: "secret", CTFID: id}),
			s.SetResults(id, 1, 1, 1),
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

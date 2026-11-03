package handlers

import (
	"database"
	"encoding/json"
	"errors"
	"fmt"
	"models"
	"os"
	"strings"
	"testing"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"gorm.io/gorm"
)

const (
	guildID     = 10
	managerRole = 100
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ctfhelper")
	if err != nil {
		panic(err)
	}
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	if err := os.Mkdir("data", 0o750); err != nil {
		panic(err)
	}
	server := models.Server{ID: guildID, RoleManagerID: managerRole}
	if err := server.Add(database.Setup().Connection()); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func addCTF(t *testing.T, serverID snowflake.ID) models.CTF {
	t.Helper()
	ctf := models.CTF{Name: t.Name(), ServerID: serverID, TextChannelID: 20}
	if err := ctf.Add(database.GetInstance().Connection()); err != nil {
		t.Fatal(err)
	}
	return ctf
}

func exists(t *testing.T, id int64) bool {
	t.Helper()
	var ctf models.CTF
	if err := ctf.GetByID(database.GetInstance().Connection(), id); err != nil {
		t.Fatal(err)
	}
	return ctf.ID == id
}

// removeRest panics on every Discord call a test does not expect: the CTFs in
// these tests have no role, event or feed message.
type removeRest struct {
	rest.Rest
	reached   bool
	deleteErr error
	replies   []string
}

func (f *removeRest) DeleteChannel(_ snowflake.ID, _ ...rest.RequestOpt) error {
	f.reached = true
	return f.deleteErr
}

func (f *removeRest) CreateFollowupMessage(_ snowflake.ID, _ string, m discord.MessageCreate, _ ...rest.RequestOpt) (*discord.Message, error) {
	f.replies = append(f.replies, m.Content)
	return &discord.Message{}, nil
}

func selectCTF(t *testing.T, f *removeRest, ctfID int64, role int) string {
	t.Helper()
	raw := fmt.Sprintf(`{"id":"1","type":3,"application_id":"2","token":"t","version":1,"guild_id":"%d",
		"channel":{"id":"20","type":0,"name":"c","guild_id":"%d"},
		"member":{"user":{"id":"30","username":"u"},"roles":["%d"],"permissions":"0"},
		"message":{"id":"5","channel_id":"20","author":{"id":"2","username":"b"}},
		"data":{"component_type":3,"custom_id":"remove_ctf_select","values":["%d"]}}`,
		guildID, guildID, role, ctfID)
	var i discord.ComponentInteraction
	if err := json.Unmarshal([]byte(raw), &i); err != nil {
		t.Fatal(err)
	}

	var reply string
	RemoveHandler().OnEvent(&events.ComponentInteractionCreate{
		GenericEvent:         events.NewGenericEvent(&bot.Client{Rest: f}, 0, 0),
		ComponentInteraction: i,
		Respond: func(_ discord.InteractionResponseType, data discord.InteractionResponseData, _ ...rest.RequestOpt) error {
			reply = data.(discord.MessageCreate).Content
			return nil
		},
	})
	if len(f.replies) > 0 {
		return f.replies[len(f.replies)-1]
	}
	return reply
}

func TestRemoveSelectRejectsNonManagers(t *testing.T) {
	ctf := addCTF(t, guildID)
	f := &removeRest{}
	if reply := selectCTF(t, f, ctf.ID, 999); !strings.Contains(reply, "required role") {
		t.Fatalf("reply = %q", reply)
	}
	if f.reached || !exists(t, ctf.ID) {
		t.Fatal("a non-manager reached the removal")
	}
}

func TestRemoveSelectRemovesForManagers(t *testing.T) {
	ctf := addCTF(t, guildID)
	f := &removeRest{}
	if reply := selectCTF(t, f, ctf.ID, managerRole); !strings.Contains(reply, "removed successfully") {
		t.Fatalf("reply = %q", reply)
	}
	if !f.reached || exists(t, ctf.ID) {
		t.Fatal("the CTF was not removed")
	}
}

func TestRemoveSelectToleratesMissingChannel(t *testing.T) {
	ctf := addCTF(t, guildID)
	f := &removeRest{deleteErr: &rest.Error{Code: rest.JSONErrorCodeUnknownChannel}}
	selectCTF(t, f, ctf.ID, managerRole)
	if exists(t, ctf.ID) {
		t.Fatal("a CTF whose channel is gone must still be removable")
	}
}

func TestRemoveSelectRemovesCTFWithoutChannel(t *testing.T) {
	db := database.GetInstance().Connection()
	ctf := models.CTF{Name: t.Name(), ServerID: guildID} // left behind by a failed /create
	if err := ctf.Add(db); err != nil {
		t.Fatal(err)
	}
	selectCTF(t, &removeRest{}, ctf.ID, managerRole)
	if exists(t, ctf.ID) {
		t.Fatal("a half-created CTF must be removable")
	}
}

func TestRemoveSelectKeepsCTFWhenDiscordFails(t *testing.T) {
	ctf := addCTF(t, guildID)
	f := &removeRest{deleteErr: errors.New("discord is down")}
	if reply := selectCTF(t, f, ctf.ID, managerRole); !strings.Contains(reply, "Error while removing") {
		t.Fatalf("reply = %q", reply)
	}
	if !exists(t, ctf.ID) {
		t.Fatal("the CTF was dropped although its channel still exists")
	}
}

func TestRemoveSelectRejectsCTFsOfOtherGuilds(t *testing.T) {
	ctf := addCTF(t, 99)
	f := &removeRest{}
	selectCTF(t, f, ctf.ID, managerRole)
	if f.reached || !exists(t, ctf.ID) {
		t.Fatal("a manager of another guild reached the removal")
	}
}

func countCreds(t *testing.T, db *gorm.DB, ctfID int64) int64 {
	t.Helper()
	var n int64
	if err := db.Unscoped().Model(&models.Creds{}).Where(models.Creds{CTFID: ctfID}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCredsDeleteErasesThePassword(t *testing.T) {
	db := database.GetInstance().Connection()
	ctf := addCTF(t, guildID)
	creds := models.Creds{Username: "u", Password: "secret", CTFID: ctf.ID}
	if err := creds.Add(db); err != nil {
		t.Fatal(err)
	}
	if err := (&models.Creds{CTFID: ctf.ID}).Delete(db); err != nil {
		t.Fatal(err)
	}
	if n := countCreds(t, db, ctf.ID); n != 0 {
		t.Fatalf("%d credential rows survive, including soft-deleted ones", n)
	}
}

func TestCTFDeleteTakesCredsAndReportWithIt(t *testing.T) {
	db := database.GetInstance().Connection()
	ctf, other := addCTF(t, guildID), models.CTF{Name: "other", ServerID: guildID}
	if err := other.Add(db); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{ctf.ID, other.ID} {
		creds := models.Creds{Username: "u", Password: "secret", CTFID: id}
		report := models.Report{CTFID: id, Solves: 1}
		if err := errors.Join(creds.Add(db), report.Add(db)); err != nil {
			t.Fatal(err)
		}
	}

	if err := ctf.Delete(db); err != nil {
		t.Fatal(err)
	}

	if n := countCreds(t, db, ctf.ID); n != 0 {
		t.Fatalf("%d credential rows survive the CTF", n)
	}
	var report models.Report
	if err := report.GetByCTFID(db, ctf.ID); err != nil || report.ID != 0 {
		t.Fatalf("report survives the CTF: %+v, err %v", report, err)
	}
	if exists(t, ctf.ID) {
		t.Fatal("CTF survives")
	}
	if countCreds(t, db, other.ID) != 1 {
		t.Fatal("credentials of another CTF were deleted")
	}
}

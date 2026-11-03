package commands

import (
	"ctftime"
	"database"
	"fmt"
	"models"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/disgoorg/snowflake/v2"
)

func seedCTF(t *testing.T, channel, ctftimeID int) models.CTF {
	t.Helper()
	ctf := models.CTF{Name: "x - 2026", ServerID: guildID, TextChannelID: snowflake.ID(channel), RoleID: ctfRole, CTFTimeID: int64(ctftimeID)}
	if err := ctf.Add(database.GetInstance().Connection()); err != nil {
		t.Fatal(err)
	}
	return ctf
}

func stubCTFtime(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	old := ctftime.BaseURL
	ctftime.BaseURL = srv.URL
	t.Cleanup(func() {
		ctftime.BaseURL = old
		srv.Close()
	})
}

func TestReportShowsCTFtimeResults(t *testing.T) {
	const channel, ctftimeID = 22, 777
	db := database.GetInstance().Connection()
	ctf := seedCTF(t, channel, ctftimeID)
	stored := models.Report{CTFID: ctf.ID, Place: -1, Score: -1, Solves: 4}
	if err := stored.Add(db); err != nil {
		t.Fatal(err)
	}
	stubCTFtime(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"%d":{"scores":[{"team_id":%d,"place":7,"points":"123.5"}]}}`, ctftimeID, teamID)
	})

	f := &fakeRest{}
	run(t, f, ReportHandler(), channel, "", ctfRole)

	if len(f.followups) != 1 || len(f.followups[0].Embeds) != 1 {
		t.Fatalf("expected the report embed, got %+v", f.followups)
	}
	embed := f.followups[0].Embeds[0]
	for i, want := range []string{"7", "123", "4"} {
		if got := embed.Fields[i].Value; got != want {
			t.Errorf("%s = %q, want %q", embed.Fields[i].Name, got, want)
		}
	}
	if embed.Timestamp.IsZero() {
		t.Error("embed timestamp is empty")
	}

	var saved models.Report
	if err := saved.GetByCTFID(db, ctf.ID); err != nil || saved.Place != 7 || saved.Score != 123 || saved.Solves != 4 {
		t.Fatalf("saved report = %+v, err %v", saved, err)
	}
}

func TestReportWithoutResultsShowsNA(t *testing.T) {
	const channel = 24
	seedCTF(t, channel, 778)
	stubCTFtime(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{}`) })

	f := &fakeRest{}
	run(t, f, ReportHandler(), channel, "", ctfRole)
	embed := f.followups[0].Embeds[0]
	if embed.Fields[0].Value != "N/A" || embed.Fields[1].Value != "N/A" {
		t.Fatalf("fields = %+v", embed.Fields)
	}
}

func TestDeleteFlagReachesZeroSolves(t *testing.T) {
	const channel = 23
	db := database.GetInstance().Connection()
	ctf := seedCTF(t, channel, 779)
	stored := models.Report{CTFID: ctf.ID, Place: -1, Score: -1, Solves: 1}
	if err := stored.Add(db); err != nil {
		t.Fatal(err)
	}

	run(t, &fakeRest{}, DeleteFlagHandler(), channel, "", managerRole)

	var saved models.Report
	if err := saved.GetByCTFID(db, ctf.ID); err != nil || saved.Solves != 0 {
		t.Fatalf("saved report = %+v, err %v", saved, err)
	}
}

func TestVoteRejectsInvalidDurationBeforeCallingCTFtime(t *testing.T) {
	stubCTFtime(t, func(http.ResponseWriter, *http.Request) { t.Error("CTFtime was called") })
	for _, hours := range []int{0, -1, maxPollHours + 1} {
		opt := fmt.Sprintf(`{"name":"duration","type":4,"value":%d}`, hours)
		wantReply(t, run(t, &fakeRest{}, VoteHandler(), ctfChannel, opt, managerRole), "must be between")
	}
}

func TestVotePostsAPublicPollAfterAcknowledging(t *testing.T) {
	// Any moment of the week the handler considers "this week".
	start := time.Now()
	if d := start.Weekday(); d == time.Saturday || d == time.Sunday {
		start = start.AddDate(0, 0, 7)
	}
	stubCTFtime(t, func(w http.ResponseWriter, _ *http.Request) {
		s := start.Format(time.RFC3339)
		fmt.Fprintf(w, `[{"id":1,"title":"a","start":%q},{"id":2,"title":"b","start":%q}]`, s, s)
	})

	f := &fakeRest{}
	wantReply(t, run(t, f, VoteHandler(), ctfChannel, "", managerRole), "Poll created")
	if len(f.messages) != 1 || f.messages[0].Poll == nil || len(f.messages[0].Poll.Answers) != 2 {
		t.Fatalf("messages = %+v", f.messages)
	}
	if f.messages[0].Poll.Duration < 1 {
		t.Fatalf("poll duration = %d", f.messages[0].Poll.Duration)
	}
}

func TestHelpListsEveryCommandWithinDiscordLimit(t *testing.T) {
	text := helpText()
	for _, want := range []string{"/init", "/flag", "role_manager", "mate"} {
		if !strings.Contains(text, want) {
			t.Errorf("help does not mention %q", want)
		}
	}
	if n := utf8.RuneCountInString(text); n > 4096 {
		t.Errorf("help is %d characters, the embed limit is 4096", n)
	}
}

func TestRemoveOptionsRespectDiscordLimits(t *testing.T) {
	long := strings.Repeat("è", 500)
	opts := ParseCtfsOptions([]models.CTF{{Name: long, Description: long}})
	if utf8.RuneCountInString(opts[0].Label) > 100 || utf8.RuneCountInString(opts[0].Description) > 100 {
		t.Fatalf("option too long: %d/%d", len(opts[0].Label), len(opts[0].Description))
	}
}

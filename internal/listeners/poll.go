package listeners

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func (l *Listeners) pollResult(e *events.MessageCreate) {
	// This runs for every message: leave before touching the database.
	if e.GuildID == nil || e.Message.Type != discord.MessageTypePollResult || e.Message.Author.ID != e.Client().ID() {
		return
	}

	server, err := l.store.Server(*e.GuildID)
	if err != nil {
		log.Error("failed to fetch server configuration", "err", err, "guild_id", *e.GuildID)
		return
	}
	if e.Message.MessageReference == nil || e.Message.MessageReference.MessageID == nil {
		log.Error("poll result has no message reference", "message_id", e.Message.ID)
		return
	}
	message, err := e.Client().Rest.GetMessage(e.ChannelID, *e.Message.MessageReference.MessageID)
	if err != nil {
		log.Error("failed to fetch poll message", "err", err, "channel_id", e.ChannelID)
		return
	}
	if message.Poll == nil || message.Poll.Results == nil {
		log.Error("message has no poll results", "message_id", message.ID)
		return
	}

	answer, err := pollWinner(*message.Poll)
	if errors.Is(err, errPollTie) {
		_, err := e.Client().Rest.CreateMessage(e.ChannelID, discord.MessageCreate{
			Content: fmt.Sprintf("<@&%s>, The last poll resulted in a tie.", server.RoleTeamID),
		})
		if err != nil {
			log.Error("failed to send tie message", "err", err)
		}
		return
	}
	if err != nil {
		log.Warn("poll has no winner", "err", err, "message_id", message.ID)
		return
	}

	ctftimeID, err := parseCTFTimeID(answer)
	if err != nil {
		log.Error("failed to parse CTFtime ID from poll answer", "err", err, "answer", answer)
		return
	}

	if err := l.ctf.Create(context.Background(), e.Client(), server, ctftimeID); err != nil {
		log.Error("failed to create the CTF that won the poll", "err", err, "ctf_time_id", ctftimeID)
		if _, err := e.Client().Rest.CreateMessage(e.ChannelID, discord.MessageCreate{
			Content: fmt.Sprintf("<@&%s>, failed to create the CTF that won the poll. ❌", server.RoleManagerID),
		}); err != nil {
			log.Error("failed to report poll failure", "err", err)
		}
	}
}

var errPollTie = errors.New("poll resulted in a tie")

// pollWinner returns the text of the most voted answer.
func pollWinner(poll discord.Poll) (string, error) {
	maxVotes := 0
	winners := make([]int, 0, len(poll.Results.AnswerCounts))
	for _, count := range poll.Results.AnswerCounts {
		switch {
		case count.Count > maxVotes:
			maxVotes = count.Count
			winners = append(winners[:0], count.ID)
		case count.Count == maxVotes && maxVotes > 0:
			winners = append(winners, count.ID)
		}
	}

	switch len(winners) {
	case 0:
		return "", errors.New("no votes")
	case 1:
	default:
		return "", errPollTie
	}

	index := winners[0] - 1 // Discord numbers answers from 1.
	if index < 0 || index >= len(poll.Answers) || poll.Answers[index].PollMedia.Text == nil {
		return "", fmt.Errorf("invalid winning answer %d", winners[0])
	}
	return *poll.Answers[index].PollMedia.Text, nil
}

// parseCTFTimeID reads the ID out of a poll answer, "Title (1234)".
func parseCTFTimeID(answer string) (int, error) {
	open, closing := strings.LastIndex(answer, "("), strings.LastIndex(answer, ")")
	if open == -1 || closing < open {
		return 0, fmt.Errorf("no ID in %q", answer)
	}
	return strconv.Atoi(strings.TrimSpace(answer[open+1 : closing]))
}

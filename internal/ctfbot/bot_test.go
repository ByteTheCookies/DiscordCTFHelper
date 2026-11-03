package ctfbot

import (
	"testing"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
)

// A slow listener must not keep the others waiting: this one only finishes once
// the next has run, which can't happen if events are dispatched one at a time.
func TestSlowListenerDoesNotBlockOthers(t *testing.T) {
	secondRan := make(chan struct{})
	finished := make(chan bool, 1)

	slow := bot.NewListenerFunc(func(*events.MessageCreate) {
		select {
		case <-secondRan:
			finished <- true
		case <-time.After(2 * time.Second):
			finished <- false
		}
	})
	second := bot.NewListenerFunc(func(*events.MessageCreate) { close(secondRan) })

	client, err := NewClient(t.Context(), "MTIzNDU2.fake.token", slow, second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close(t.Context()) })

	client.EventManager.DispatchEvent(&events.MessageCreate{})
	if !<-finished {
		t.Fatal("events are handled one at a time")
	}
}

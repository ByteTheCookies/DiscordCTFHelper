package main

import (
	"context"
	"log/slog"
	"testing"

	"github.com/charmbracelet/log"

	"github.com/ByteTheCookies/DiscordCTFHelper/internal/config"
)

// disgo logs through slog, so slog must follow LOG_LEVEL too.
func TestSetupLoggerAppliesToSlog(t *testing.T) {
	old := slog.Default()
	t.Cleanup(func() { slog.SetDefault(old) })

	// Debug, because slog's own default handler already enables info and above.
	setupLogger(config.LogConfig{Level: log.DebugLevel, Format: "text"})
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		t.Error("slog ignores a debug LOG_LEVEL")
	}

	setupLogger(config.LogConfig{Level: log.ErrorLevel, Format: "text"})
	if slog.Default().Enabled(context.Background(), slog.LevelInfo) {
		t.Error("slog ignores an error LOG_LEVEL")
	}
}

package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("GUILDS", "1,not-a-number,2")
	t.Setenv("DB_PATH", "")
	t.Setenv("GREET_EMOJI_ID", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Bot.DevGuilds) != 2 {
		t.Errorf("guilds = %v", cfg.Bot.DevGuilds)
	}
	if cfg.DatabasePath != "data/ctfs.db" {
		t.Errorf("database path = %q", cfg.DatabasePath)
	}
	if cfg.GreetEmojiID != 0 {
		t.Errorf("greet emoji = %d", cfg.GreetEmojiID)
	}

	t.Setenv("DB_PATH", "/tmp/x.db")
	t.Setenv("GREET_EMOJI_ID", "123")
	if cfg, err = Load(); err != nil || cfg.DatabasePath != "/tmp/x.db" || cfg.GreetEmojiID != 123 {
		t.Errorf("cfg = %+v, err %v", cfg, err)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := map[string][2]string{
		"log level":   {"LOG_LEVEL", "nonsense"},
		"greet emoji": {"GREET_EMOJI_ID", "not-a-number"},
	}
	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", "info")
			t.Setenv("GREET_EMOJI_ID", "")
			t.Setenv(env[0], env[1])
			if _, err := Load(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

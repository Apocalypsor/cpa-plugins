package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestPluginRegistrationUsesBuildVersion(t *testing.T) {
	got := pluginRegistration().Metadata.Version
	if got != pluginVersion {
		t.Fatalf("registration version = %q, build version = %q", got, pluginVersion)
	}
	if want := os.Getenv("PLUGIN_VERSION"); want != "" && got != want {
		t.Fatalf("registration version = %q, want injected version %q", got, want)
	}
}

func TestParseSettings(t *testing.T) {
	cfg := parseSettings([]byte(`
telegram_bot_token: "123:abc"
telegram_chat_id: '456'
cooldown_seconds: 60
`))
	if cfg.BotToken != "123:abc" || cfg.ChatID != "456" || cfg.Cooldown != time.Minute {
		t.Fatalf("unexpected settings: %#v", cfg)
	}
}

func TestReserveNotificationThrottlesByKey(t *testing.T) {
	state.Lock()
	state.lastSent = map[string]time.Time{}
	state.Unlock()

	now := time.Unix(1000, 0)
	if !reserveNotification("codex/a", now, time.Minute, false) {
		t.Fatal("first notification should be reserved")
	}
	if reserveNotification("codex/a", now.Add(30*time.Second), time.Minute, false) {
		t.Fatal("duplicate notification should be throttled")
	}
	if !reserveNotification("codex/a", now.Add(61*time.Second), time.Minute, false) {
		t.Fatal("notification after cooldown should be reserved")
	}
}

func TestReserveNotificationOnce(t *testing.T) {
	state.Lock()
	state.lastSent = map[string]time.Time{}
	state.Unlock()

	now := time.Unix(1000, 0)
	if !reserveNotification("email:a@example.com", now, time.Nanosecond, true) {
		t.Fatal("first email notification should be reserved")
	}
	if reserveNotification("email:a@example.com", now.Add(time.Hour), time.Nanosecond, true) {
		t.Fatal("email notification should only be reserved once")
	}
}

func TestTelegramMessageIncludesAccountFields(t *testing.T) {
	msg := telegramMessage(usageRecord{
		Provider:  "codex",
		Model:     "gpt-5-codex",
		AuthIndex: "codex-user.json",
		Source:    "test",
		Failure:   usageFailure{StatusCode: 401, Body: "unauthorized"},
	}, "user@example.com")
	for _, want := range []string{"CPA account 401", "Provider: codex", "Email: user@example.com", "Auth: codex-user.json", "Error: unauthorized"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q:\n%s", want, msg)
		}
	}
}

func TestNotificationKeyUsesEmail(t *testing.T) {
	key, once := notificationKey(usageRecord{Provider: "codex", AuthIndex: "a"}, "User@Example.COM")
	if key != "email:user@example.com" || !once {
		t.Fatalf("key=%q once=%v, want email key once", key, once)
	}
	key, once = notificationKey(usageRecord{Provider: "codex", AuthIndex: "a"}, "")
	if key != "account:codex/a/" || once {
		t.Fatalf("key=%q once=%v, want account fallback", key, once)
	}
}

func TestPluginRegistrationExposesConfigFields(t *testing.T) {
	reg := pluginRegistration()
	if !reg.Capabilities.UsagePlugin {
		t.Fatalf("capabilities = %#v, want usage", reg.Capabilities)
	}
	if len(reg.Metadata.ConfigFields) != 3 {
		t.Fatalf("config fields = %d, want 3", len(reg.Metadata.ConfigFields))
	}
	for _, want := range []string{"telegram_bot_token", "telegram_chat_id", "cooldown_seconds"} {
		found := false
		for _, field := range reg.Metadata.ConfigFields {
			found = found || field.Name == want
		}
		if !found {
			t.Fatalf("config field %q missing: %#v", want, reg.Metadata.ConfigFields)
		}
	}
}

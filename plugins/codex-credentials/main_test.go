package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestParseJWTClaimsExtractsTeamPlanAccount(t *testing.T) {
	payload := map[string]any{
		"email": "user@example.com",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct_123",
			"chatgpt_plan_type":  "team",
			"organizations": []map[string]any{
				{"title": "Other", "is_default": false},
				{"title": "Team Alpha", "is_default": true},
			},
		},
	}
	raw, _ := json.Marshal(payload)
	token := "x." + base64.RawURLEncoding.EncodeToString(raw) + ".y"

	claims, err := parseJWTClaims(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Email != "user@example.com" || claims.Auth.Plan != "team" || claims.Auth.AccountID != "acct_123" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if got := defaultOrgTitle(claims); got != "Team Alpha" {
		t.Fatalf("team = %q, want Team Alpha", got)
	}
}

func TestCredentialStateFlagsMissingRefreshAsLogin(t *testing.T) {
	row := accountRow{Status: "active", Issue: "missing refresh token"}
	if valid, login, issue := credentialState(row); valid || !login || issue == "" {
		t.Fatalf("valid=%v login=%v issue=%q, want login for missing refresh", valid, login, issue)
	}

	row = accountRow{Status: "error", StatusMessage: "unauthorized"}
	if valid, login, issue := credentialState(row); valid || !login || issue != "unauthorized" {
		t.Fatalf("valid=%v login=%v issue=%q, want login for unauthorized", valid, login, issue)
	}
}

func TestCredentialStateAcceptsExpiredAccessTokenWithRefresh(t *testing.T) {
	row := accountRow{Status: "active"}
	enrichRowFromJSON(&row, []byte(`{"type":"codex","refresh_token":"rt","expired":"2000-01-01T00:00:00Z"}`))
	if valid, login, issue := credentialState(row); !valid || login || issue != "" {
		t.Fatalf("valid=%v login=%v issue=%q, want valid refreshable credential", valid, login, issue)
	}
}

func TestCredentialStateAcceptsFreshActiveCredential(t *testing.T) {
	row := accountRow{Status: "active"}
	enrichRowFromJSON(&row, []byte(`{"type":"codex","refresh_token":"rt","expired":"2999-01-01T00:00:00Z"}`))
	if valid, login, issue := credentialState(row); !valid || login || issue != "" {
		t.Fatalf("valid=%v login=%v issue=%q, want valid", valid, login, issue)
	}
}

func TestCredentialStateRejectsUnavailableAndDisabled(t *testing.T) {
	row := accountRow{Status: "error", StatusMessage: "quota exhausted", Unavailable: true}
	if valid, login, issue := credentialState(row); valid || login || issue != "quota exhausted" {
		t.Fatalf("valid=%v login=%v issue=%q, want unavailable quota error", valid, login, issue)
	}

	row = accountRow{Status: "disabled", StatusMessage: "disabled", Disabled: true}
	if valid, login, issue := credentialState(row); valid || login || issue != "disabled" {
		t.Fatalf("valid=%v login=%v issue=%q, want disabled account unavailable", valid, login, issue)
	}
}

func TestLatestUsage401OverridesCredentialState(t *testing.T) {
	resetLastRequests(t)
	now := time.Now()
	raw, _ := json.Marshal(usageRecord{
		Provider:    "codex",
		AuthIndex:   "auth-a",
		RequestedAt: now,
		Failed:      true,
		Failure:     usageFailure{StatusCode: http.StatusUnauthorized},
	})
	if err := handleUsage(raw); err != nil {
		t.Fatal(err)
	}
	row := accountRow{AuthIndex: "auth-a", Status: "active"}
	row.Valid, row.Login, row.Issue = credentialState(row)
	applyLastRequestState(&row)
	if row.Valid || !row.Login || row.Issue != "latest request HTTP 401" {
		t.Fatalf("row = valid:%v login:%v issue:%q, want latest 401 login", row.Valid, row.Login, row.Issue)
	}

	raw, _ = json.Marshal(usageRecord{Provider: "codex", AuthIndex: "auth-a", RequestedAt: now.Add(time.Second)})
	if err := handleUsage(raw); err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(usageRecord{
		Provider:    "codex",
		AuthIndex:   "auth-a",
		RequestedAt: now.Add(-time.Second),
		Failed:      true,
		Failure:     usageFailure{StatusCode: http.StatusUnauthorized},
	})
	if err := handleUsage(raw); err != nil {
		t.Fatal(err)
	}
	row = accountRow{AuthIndex: "auth-a", Status: "active"}
	row.Valid, row.Login, row.Issue = credentialState(row)
	applyLastRequestState(&row)
	if !row.Valid || row.Login || row.Issue != "" {
		t.Fatalf("row = valid:%v login:%v issue:%q, want success to clear latest 401 overlay", row.Valid, row.Login, row.Issue)
	}
}

func TestManagementRegistersAccountsRouteAndIndexResource(t *testing.T) {
	raw, err := handleMethod("management.register", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "/plugins/codex-credentials/accounts") || !strings.Contains(string(raw), "/index.html") {
		t.Fatalf("management registration missing route/resource: %s", raw)
	}
}

func TestPluginRegistersManagementAndUsageCapabilities(t *testing.T) {
	raw, err := handleMethod("plugin.register", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"management_api":true`, `"usage_plugin":true`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("plugin registration missing %s: %s", want, raw)
		}
	}
}

func TestFormatTime(t *testing.T) {
	now := time.Date(2026, 7, 4, 1, 2, 3, 0, time.UTC)
	if got := formatTime(now); got != "2026-07-04T01:02:03Z" {
		t.Fatalf("formatTime = %q", got)
	}
}

func resetLastRequests(t *testing.T) {
	t.Helper()
	lastRequests.Lock()
	previous := lastRequests.byAuthIndex
	lastRequests.byAuthIndex = map[string]lastRequestState{}
	lastRequests.Unlock()
	t.Cleanup(func() {
		lastRequests.Lock()
		lastRequests.byAuthIndex = previous
		lastRequests.Unlock()
	})
}

package main

import (
	"encoding/base64"
	"encoding/json"
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

func TestCredentialStateUnavailableQuotaDoesNotOfferLogin(t *testing.T) {
	row := accountRow{Status: "error", StatusMessage: "quota exhausted", Unavailable: true}
	if valid, login, issue := credentialState(row); valid || login || issue != "quota exhausted" {
		t.Fatalf("valid=%v login=%v issue=%q, want unavailable without login", valid, login, issue)
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

func TestFormatTime(t *testing.T) {
	now := time.Date(2026, 7, 4, 1, 2, 3, 0, time.UTC)
	if got := formatTime(now); got != "2026-07-04T01:02:03Z" {
		t.Fatalf("formatTime = %q", got)
	}
}

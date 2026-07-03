package main

import (
	"encoding/json"
	"testing"
)

func TestChooseMasterPrefersRefreshTokenThenLatest(t *testing.T) {
	old := &authFile{Name: "old.json", Rec: map[string]any{
		"email":         "a@example.com",
		"refresh_token": "rt-old",
		"last_refresh":  "2026-01-01T00:00:00Z",
	}}
	latestNoRefresh := &authFile{Name: "latest-no-rt.json", Rec: map[string]any{
		"email":        "a@example.com",
		"last_refresh": "2026-01-03T00:00:00Z",
	}}
	newer := &authFile{Name: "newer.json", Rec: map[string]any{
		"email":         "a@example.com",
		"refresh_token": "rt-new",
		"last_refresh":  "2026-01-02T00:00:00Z",
	}}

	got := chooseMaster([]*authFile{old, latestNoRefresh, newer}, nil)
	if got != newer {
		t.Fatalf("master = %s, want %s", got.Name, newer.Name)
	}
}

func TestPlanUpdatesCopiesAccessFieldsAndClearsSiblingRefreshToken(t *testing.T) {
	master := &authFile{Name: "master.json", Rec: map[string]any{
		"access_token":  "access",
		"id_token":      "id",
		"expired":       false,
		"last_refresh":  "2026-01-02T00:00:00Z",
		"refresh_token": "master-rt",
		"account_id":    "workspace-master",
	}}
	sibling := &authFile{Name: "sibling.json", Rec: map[string]any{
		"access_token":  "old",
		"id_token":      "old-id",
		"expired":       true,
		"last_refresh":  "2026-01-01T00:00:00Z",
		"refresh_token": "sibling-rt",
		"account_id":    "workspace-sibling",
	}}

	plans := planUpdates(master, []*authFile{master, sibling})
	if len(plans) != 1 {
		t.Fatalf("plans = %d, want 1", len(plans))
	}
	changed := plans[0].Changed
	if changed["account_id"] != nil {
		t.Fatal("account_id must not be copied")
	}
	if changed["access_token"] != "access" || changed["id_token"] != "id" || changed["refresh_token"] != "" {
		t.Fatalf("unexpected changes: %#v", changed)
	}
}

func TestManagementRegistersFanoutRouteAndIndexResource(t *testing.T) {
	raw, err := handleMethod("management.register", nil)
	if err != nil {
		t.Fatal(err)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	var reg managementRegistration
	if err := json.Unmarshal(env.Result, &reg); err != nil {
		t.Fatal(err)
	}
	if len(reg.Routes) != 1 || reg.Routes[0].Path != "/plugins/codex-fanout/fanout" {
		t.Fatalf("unexpected routes: %#v", reg.Routes)
	}
	if len(reg.Resources) != 1 || reg.Resources[0].Path != "/index.html" {
		t.Fatalf("unexpected resources: %#v", reg.Resources)
	}
}

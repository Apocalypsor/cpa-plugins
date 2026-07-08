package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} cliproxy_buffer;

typedef int (*cliproxy_host_call_fn)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_host_free_fn)(void*, size_t);

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	cliproxy_host_call_fn call;
	cliproxy_host_free_fn free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);

static const cliproxy_host_api* stored_host;

static void store_host_api(const cliproxy_host_api* host) {
	stored_host = host;
}

static int call_host_api(const char* method, const uint8_t* request, size_t request_len, cliproxy_buffer* response) {
	if (stored_host == NULL || stored_host->call == NULL) {
		return 1;
	}
	return stored_host->call(stored_host->host_ctx, method, request, request_len, response);
}

static void free_host_buffer(void* ptr, size_t len) {
	if (stored_host != NULL && stored_host->free_buffer != NULL && ptr != NULL) {
		stored_host->free_buffer(ptr, len);
	}
}
*/
import "C"

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"
)

const (
	abiVersion    uint32 = 1
	schemaVersion uint32 = 1
	pluginName           = "codex-credentials"
	pluginVersion        = "0.4.0"
)

const indexHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Codex Credentials</title>
  <style>
    :root{color-scheme:light dark;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;--primary-color:#4f46e5;--bg-primary:#fff;--bg-secondary:#f6f7fb;--bg-tertiary:#eef1f6;--text-primary:#111827;--text-secondary:#6b7280;--text-tertiary:#9ca3af;--border-color:#d9dee8;--error-color:#dc2626;--ok-color:#16a34a;--warn-color:#d97706;--radius-md:8px;--radius-lg:12px}
    body{margin:0;background:var(--bg-secondary);color:var(--text-primary)}
    main{max-width:1420px;margin:0 auto;padding:28px 20px}
    h1{font-size:28px;font-weight:700;margin:0}
    label{display:block;font-weight:650;margin:0 0 6px;color:var(--text-secondary);font-size:13px}
    input{box-sizing:border-box;width:100%;font:inherit;padding:10px 12px;border:1px solid var(--border-color);border-radius:var(--radius-md);background:var(--bg-primary);color:var(--text-primary)}
    button{font:inherit;font-weight:700;border:0;border-radius:var(--radius-md);padding:10px 14px;background:var(--primary-color);color:white;cursor:pointer;white-space:nowrap}
    button.secondary{background:var(--bg-tertiary);color:var(--text-primary);border:1px solid var(--border-color)}
    button:disabled{opacity:.55;cursor:not-allowed}
    .page-head{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:18px;flex-wrap:wrap}
    .toolbar{display:flex;align-items:end;gap:12px;margin-bottom:14px;flex-wrap:wrap}
    .key{min-width:280px;flex:1}
    .row{display:flex;gap:10px;align-items:center;flex-wrap:wrap}
    .status{padding:10px 12px;border-radius:var(--radius-md);font-size:14px;margin:14px 0;background:rgba(79,70,229,.12);color:var(--primary-color)}
    .status.success{background:rgba(22,163,74,.12);color:var(--ok-color)}
    .status.error{background:rgba(220,38,38,.12);color:var(--error-color)}
    .box{background:var(--bg-primary);border:1px dashed var(--border-color);border-radius:var(--radius-lg);padding:14px;margin:14px 0}
    dialog{border:0;padding:0;background:transparent;color:var(--text-primary);width:min(720px,calc(100vw - 32px))}
    dialog::backdrop{background:rgba(15,23,42,.45)}
    dialog .box{margin:0;border-style:solid;box-shadow:0 18px 60px rgba(15,23,42,.22)}
    .modal-head{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:12px;font-weight:800}
    .url{font-weight:700;word-break:break-all;overflow-wrap:anywhere;line-height:1.5}
    .cards{display:grid;grid-template-columns:repeat(auto-fill,minmax(340px,1fr));gap:18px}
    .cred-card{background:var(--bg-primary);border:1px solid var(--border-color);border-radius:var(--radius-lg);padding:18px;box-shadow:0 1px 2px rgba(15,23,42,.04);display:flex;flex-direction:column;gap:14px;min-height:260px}
    .card-top{display:flex;align-items:center;justify-content:space-between;gap:12px}
    .badges{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
    .badge{display:inline-flex;align-items:center;border-radius:7px;padding:4px 10px;font-size:12px;font-weight:800;background:var(--bg-tertiary);color:var(--text-secondary);border:1px solid var(--border-color)}
    .badge.codex{background:rgba(79,70,229,.18);border-color:rgba(79,70,229,.28);color:var(--primary-color)}
    .badge.ok{background:rgba(22,163,74,.14);border-color:rgba(22,163,74,.24);color:var(--ok-color)}
    .badge.bad{background:rgba(220,38,38,.14);border-color:rgba(220,38,38,.24);color:var(--error-color)}
    .badge.warn{background:rgba(217,119,6,.14);border-color:rgba(217,119,6,.24);color:var(--warn-color)}
    .title{font-size:18px;font-weight:800;line-height:1.35;overflow-wrap:anywhere}
    .workspace{font-size:14px;font-weight:700;color:var(--text-secondary);overflow-wrap:anywhere}
    .meta{display:flex;align-items:center;gap:12px;flex-wrap:wrap;color:var(--text-secondary);font-size:13px}
    .stat{display:inline-flex;align-items:center;gap:6px;border-radius:999px;padding:4px 10px;background:var(--bg-tertiary);font-weight:800}
    .stat.ok{color:var(--ok-color)}
    .stat.bad{color:#b91c1c}
    .health-head{display:flex;align-items:center;justify-content:space-between;color:var(--text-secondary);font-size:13px;font-weight:700}
    .blocks{display:grid;grid-template-columns:repeat(20,1fr);gap:5px}
    .block{height:8px;border-radius:999px;background:var(--bg-tertiary)}
    .block.ok{background:var(--ok-color)}
    .block.bad{background:var(--error-color)}
    .issue{font-size:13px;color:var(--text-secondary);min-height:18px;overflow-wrap:anywhere}
    .card-actions{margin-top:auto;display:flex;align-items:center;justify-content:space-between;gap:12px;border-top:1px dashed var(--border-color);padding-top:14px}
    .last{font-size:12px;color:var(--text-tertiary);overflow-wrap:anywhere}
    .empty{grid-column:1/-1;padding:22px;border:1px dashed var(--border-color);border-radius:var(--radius-lg);background:var(--bg-primary);color:var(--text-secondary)}
    @media (prefers-color-scheme:dark){:root{--bg-primary:#151b29;--bg-secondary:#0b1020;--bg-tertiary:#242c3d;--text-primary:#e5e7eb;--text-secondary:#a7b0bf;--text-tertiary:#777f90;--border-color:#30384a}}
    @media (max-width:760px){main{padding:20px 14px}.toolbar{align-items:stretch}.key{min-width:100%}.cards{grid-template-columns:1fr}.card-actions{align-items:stretch;flex-direction:column}.card-actions button{width:100%}}
  </style>
</head>
<body>
<main>
  <div class="page-head">
    <h1>Codex Credentials</h1>
    <button id="refresh" class="secondary">Refresh</button>
  </div>
  <section class="toolbar">
    <div class="key">
      <label for="key">CPA management key</label>
      <input id="key" type="password" autocomplete="off" placeholder="remote-management.secret-key">
    </div>
    <button id="login" class="secondary">Login new account</button>
  </section>
  <dialog id="authDialog">
    <div class="box">
      <div class="modal-head">
        <span>Login account</span>
        <button id="closeAuth" class="secondary" type="button">Close</button>
      </div>
      <div id="authUrl" class="url"></div>
      <div class="row" style="margin-top:10px">
        <button id="openAuth" class="secondary">Open link</button>
        <button id="copyAuth" class="secondary">Copy link</button>
      </div>
      <label for="callback" style="margin-top:12px">Callback URL</label>
      <input id="callback" autocomplete="off" placeholder="http://localhost:1455/auth/callback?code=...&state=...">
      <div class="row" style="margin-top:10px">
        <button id="submitCallback">Verify callback</button>
      </div>
    </div>
  </dialog>
  <div id="status" class="status" hidden></div>
  <section id="cards" class="cards"><div class="empty">Ready.</div></section>
</main>
<script>
const ENC_PREFIX = "enc::v1::";
const SECRET_SALT = "cli-proxy-api-webui::secure-storage";
const key = document.getElementById("key");
const refresh = document.getElementById("refresh");
const login = document.getElementById("login");
const cards = document.getElementById("cards");
const statusBox = document.getElementById("status");
const authDialog = document.getElementById("authDialog");
const authUrl = document.getElementById("authUrl");
const closeAuth = document.getElementById("closeAuth");
const openAuth = document.getElementById("openAuth");
const copyAuth = document.getElementById("copyAuth");
const callback = document.getElementById("callback");
const submitCallback = document.getElementById("submitCallback");

let currentAuthURL = "";
let currentState = "";
let activeAuthIndex = "";
let accounts = [];
let pollTimer = 0;

function xorBytes(data, keyBytes) {
  const result = new Uint8Array(data.length);
  for (let i = 0; i < data.length; i++) result[i] = data[i] ^ keyBytes[i % keyBytes.length];
  return result;
}

function deobfuscate(payload) {
  if (!payload || !payload.startsWith(ENC_PREFIX)) return payload;
  const keyBytes = new TextEncoder().encode(SECRET_SALT + "|" + window.location.host + "|" + navigator.userAgent);
  const raw = Uint8Array.from(atob(payload.slice(ENC_PREFIX.length)), c => c.charCodeAt(0));
  return new TextDecoder().decode(xorBytes(raw, keyBytes));
}

function readSavedManagementKey() {
  for (const name of ["cli-proxy-auth", "managementKey"]) {
    const raw = localStorage.getItem(name);
    if (!raw) continue;
    try {
      const parsed = JSON.parse(deobfuscate(raw));
      if (typeof parsed === "string") return parsed;
      const saved = parsed && (parsed.state || parsed);
      if (saved && typeof saved.managementKey === "string") return saved.managementKey;
    } catch {}
  }
  return "";
}

function token() {
  const value = key.value.trim();
  if (!value) {
    setStatus("Missing CPA management key.", "error");
    key.focus();
    throw new Error("missing key");
  }
  return value;
}

function headers(json) {
  const h = {"Authorization": "Bearer " + token()};
  if (json) h["Content-Type"] = "application/json";
  return h;
}

async function readJSON(res) {
  const text = await res.text();
  let data = {};
  try { data = text ? JSON.parse(text) : {}; } catch { data = {error: text}; }
  if (!res.ok) throw new Error(data.error || data.message || data.output || ("HTTP " + res.status));
  return data;
}

function setStatus(text, kind) {
  statusBox.hidden = false;
  statusBox.textContent = text;
  statusBox.className = "status" + (kind ? " " + kind : "");
}

function setAuthURL(url) {
  currentAuthURL = url || "";
  authUrl.textContent = currentAuthURL;
  if (currentAuthURL) authDialog.showModal();
  else if (authDialog.open) authDialog.close();
}

function node(tag, cls, text) {
  const el = document.createElement(tag);
  if (cls) el.className = cls;
  if (text !== undefined) el.textContent = text;
  return el;
}

function statusLabel(item) {
  if (item.valid) return "valid";
  return "unavailable";
}

function statusClass(item) {
  if (item.valid) return "ok";
  return item.login ? "bad" : "warn";
}

function accountUUID(item) {
  return String(item.account_id || item.account || item.auth_index || "uuid unavailable").trim();
}

function healthPercent(item) {
  const ok = Number(item.success || 0);
  const failed = Number(item.failed || 0);
  const total = ok + failed;
  return total > 0 ? Math.round(ok * 1000 / total) / 10 : null;
}

function renderBlocks(item) {
  const wrap = node("div", "blocks");
  const recent = Array.isArray(item.recent_requests) ? item.recent_requests.slice(-20) : [];
  if (recent.length) {
    for (const bucket of recent) {
      const b = node("span", "block");
      if ((bucket.failed || 0) > 0) b.classList.add("bad");
      else if ((bucket.success || 0) > 0) b.classList.add("ok");
      wrap.appendChild(b);
    }
    for (let i = recent.length; i < 20; i++) wrap.appendChild(node("span", "block"));
    return wrap;
  }
  const percent = healthPercent(item);
  const active = percent === null ? 0 : Math.round(percent / 5);
  const failMarker = item.failed ? Math.min(19, Math.max(0, active - 1)) : -1;
  for (let i = 0; i < 20; i++) {
    const b = node("span", "block");
    if (i < active) b.classList.add("ok");
    if (i === failMarker) b.classList.add("bad");
    wrap.appendChild(b);
  }
  return wrap;
}

function render(items) {
  cards.replaceChildren();
  if (!items.length) {
    const empty = node("div", "empty", "No Codex credentials found.");
    const btn = node("button", "secondary", "Login account");
    btn.style.marginTop = "12px";
    btn.addEventListener("click", () => startOAuth(""));
    empty.appendChild(document.createElement("br"));
    empty.appendChild(btn);
    cards.appendChild(empty);
    return;
  }
  for (const item of items) {
    const card = node("article", "cred-card");
    card.dataset.authIndex = item.auth_index || "";

    const top = node("div", "card-top");
    const badges = node("div", "badges");
    badges.appendChild(node("span", "badge codex", "Codex"));
    if (item.plan) badges.appendChild(node("span", "badge", item.plan));
    badges.appendChild(node("span", "badge " + statusClass(item), statusLabel(item)));
    top.appendChild(badges);
    card.appendChild(top);

    card.appendChild(node("div", "title", item.email || item.name || "unknown account"));
    card.appendChild(node("div", "workspace", "Team " + accountUUID(item)));

    const meta = node("div", "meta");
    meta.appendChild(node("span", "stat ok", "success " + String(item.success || 0)));
    meta.appendChild(node("span", "stat bad", "failed " + String(item.failed || 0)));
    card.appendChild(meta);

    const percent = healthPercent(item);
    const health = node("div", "health");
    const healthHead = node("div", "health-head");
    healthHead.appendChild(node("span", "", "health"));
    healthHead.appendChild(node("span", "", percent === null ? "-" : String(percent) + "%"));
    health.appendChild(healthHead);
    health.appendChild(renderBlocks(item));
    card.appendChild(health);

    card.appendChild(node("div", "issue", item.issue || item.status_message || ""));

    const actions = node("div", "card-actions");
    actions.appendChild(node("div", "last", item.last_refresh ? "last refresh " + item.last_refresh : "last refresh -"));
    const btn = node("button", item.login ? "" : "secondary", "Login account");
    btn.addEventListener("click", () => startOAuth(item.auth_index || "", btn));
    actions.appendChild(btn);
    card.appendChild(actions);

    cards.appendChild(card);
  }
}

function mergeAccounts(updated) {
  const byIndex = new Map((updated || []).map(item => [item.auth_index, item]));
  accounts = accounts.map(item => byIndex.get(item.auth_index) || item);
  for (const item of updated || []) {
    if (!accounts.some(existing => existing.auth_index === item.auth_index)) accounts.push(item);
  }
}

async function loadAccounts() {
  refresh.disabled = true;
  try {
    const data = await readJSON(await fetch("/v0/management/plugins/codex-credentials/accounts", {headers: headers(false)}));
    accounts = data.accounts || [];
    render(accounts);
    setStatus("Loaded " + accounts.length + " credentials.", "success");
  } catch (err) {
    if (err.message !== "missing key") setStatus(String(err), "error");
  } finally {
    refresh.disabled = false;
  }
}

async function refreshAccount(authIndex) {
  if (!authIndex) {
    await loadAccounts();
    return;
  }
  const query = new URLSearchParams({auth_index: authIndex});
  const data = await readJSON(await fetch("/v0/management/plugins/codex-credentials/accounts?" + query, {headers: headers(false)}));
  const updated = data.accounts || [];
  if (!updated.length) {
    await loadAccounts();
    return;
  }
  mergeAccounts(updated);
  render(accounts);
  setStatus("Credential refreshed.", "success");
}

async function startOAuth(authIndex, sourceButton) {
  activeAuthIndex = authIndex || "";
  login.disabled = true;
  if (sourceButton) sourceButton.disabled = true;
  setAuthURL("");
  currentState = "";
  if (pollTimer) window.clearInterval(pollTimer);
  setStatus("Generating Codex OAuth link...", "");
  try {
    const data = await readJSON(await fetch("/v0/management/codex-auth-url?is_webui=true", {headers: headers(false)}));
    if (!data.url || !data.state) throw new Error("CPA did not return an auth URL/state");
    currentState = data.state;
    setAuthURL(data.url);
    setStatus("OAuth link generated.", "");
  } catch (err) {
    setStatus("OAuth start failed: " + err.message, "error");
  } finally {
    login.disabled = false;
    if (sourceButton) sourceButton.disabled = false;
  }
}

function startPolling(state) {
  const deadline = Date.now() + 5 * 60 * 1000;
  pollTimer = window.setInterval(async () => {
    if (Date.now() > deadline) {
      window.clearInterval(pollTimer);
      pollTimer = 0;
      submitCallback.disabled = false;
      setStatus("OAuth timed out.", "error");
      return;
    }
    try {
      const data = await readJSON(await fetch("/v0/management/get-auth-status?state=" + encodeURIComponent(state), {headers: headers(false)}));
      if (data.status === "wait") return;
      window.clearInterval(pollTimer);
      pollTimer = 0;
      submitCallback.disabled = false;
      if (data.status === "ok") {
        setStatus("Authentication succeeded. Refreshing credential...", "success");
        setAuthURL("");
        callback.value = "";
        const target = activeAuthIndex;
        activeAuthIndex = "";
        currentState = "";
        await refreshAccount(target);
      } else {
        setStatus("Authentication failed: " + (data.error || "unknown error"), "error");
      }
    } catch (err) {
      window.clearInterval(pollTimer);
      pollTimer = 0;
      submitCallback.disabled = false;
      setStatus("OAuth polling failed: " + err.message, "error");
    }
  }, 3000);
}

async function sendCallback() {
  const redirectURL = callback.value.trim();
  if (!redirectURL) {
    setStatus("Missing callback URL.", "error");
    callback.focus();
    return;
  }
  if (!currentState) {
    setStatus("Generate an OAuth link first.", "error");
    return;
  }
  submitCallback.disabled = true;
  try {
    await readJSON(await fetch("/v0/management/oauth-callback", {
      method: "POST",
      headers: headers(true),
      body: JSON.stringify({provider: "codex", redirect_url: redirectURL, state: currentState})
    }));
    setStatus("Callback submitted. Waiting for authentication...", "");
    startPolling(currentState);
  } catch (err) {
    setStatus("Callback submit failed: " + err.message, "error");
  }
  if (!pollTimer) submitCallback.disabled = false;
}

key.value = readSavedManagementKey();
refresh.addEventListener("click", loadAccounts);
login.addEventListener("click", () => startOAuth(""));
closeAuth.addEventListener("click", () => authDialog.close());
openAuth.addEventListener("click", () => currentAuthURL && window.open(currentAuthURL, "_blank"));
copyAuth.addEventListener("click", () => currentAuthURL && navigator.clipboard.writeText(currentAuthURL));
submitCallback.addEventListener("click", sendCallback);
if (key.value) loadAccounts();
</script>
</body>
</html>`

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type registration struct {
	SchemaVersion uint32                   `json:"schema_version"`
	Metadata      metadata                 `json:"metadata"`
	Capabilities  registrationCapabilities `json:"capabilities"`
}

type metadata struct {
	Name             string        `json:"Name"`
	Version          string        `json:"Version"`
	Author           string        `json:"Author"`
	GitHubRepository string        `json:"GitHubRepository"`
	Logo             string        `json:"Logo"`
	ConfigFields     []configField `json:"ConfigFields"`
}

type configField struct {
	Name        string   `json:"Name"`
	Type        string   `json:"Type"`
	EnumValues  []string `json:"EnumValues"`
	Description string   `json:"Description"`
}

type registrationCapabilities struct {
	ManagementAPI bool `json:"management_api"`
	UsagePlugin   bool `json:"usage_plugin"`
}

type managementRegistration struct {
	Routes    []managementRoute    `json:"Routes,omitempty"`
	Resources []managementResource `json:"Resources,omitempty"`
}

type managementRoute struct {
	Method      string `json:"Method"`
	Path        string `json:"Path"`
	Description string `json:"Description,omitempty"`
}

type managementResource struct {
	Path        string `json:"Path"`
	Menu        string `json:"Menu,omitempty"`
	Description string `json:"Description,omitempty"`
}

type managementRequest struct {
	Method string `json:"Method"`
	Path   string `json:"Path"`
	Query  url.Values
}

type managementResponse struct {
	StatusCode int         `json:"StatusCode,omitempty"`
	Headers    http.Header `json:"Headers,omitempty"`
	Body       []byte      `json:"Body,omitempty"`
}

type hostAuthFileEntry struct {
	ID             string               `json:"id,omitempty"`
	AuthIndex      string               `json:"auth_index,omitempty"`
	Name           string               `json:"name"`
	Label          string               `json:"label,omitempty"`
	Type           string               `json:"type,omitempty"`
	Provider       string               `json:"provider,omitempty"`
	Status         string               `json:"status,omitempty"`
	StatusMessage  string               `json:"status_message,omitempty"`
	Disabled       bool                 `json:"disabled,omitempty"`
	Unavailable    bool                 `json:"unavailable,omitempty"`
	RuntimeOnly    bool                 `json:"runtime_only,omitempty"`
	LastRefresh    time.Time            `json:"last_refresh,omitempty"`
	NextRetryAfter time.Time            `json:"next_retry_after,omitempty"`
	Email          string               `json:"email,omitempty"`
	AccountType    string               `json:"account_type,omitempty"`
	Account        string               `json:"account,omitempty"`
	Success        int64                `json:"success,omitempty"`
	Failed         int64                `json:"failed,omitempty"`
	RecentRequests []recentRequestEntry `json:"recent_requests,omitempty"`
}

type recentRequestEntry struct {
	Time    string `json:"time"`
	Success int64  `json:"success"`
	Failed  int64  `json:"failed"`
}

type hostAuthListResponse struct {
	Files []hostAuthFileEntry `json:"files"`
}

type hostAuthGetRequest struct {
	AuthIndex string `json:"auth_index"`
}

type hostAuthGetResponse struct {
	AuthIndex string          `json:"auth_index"`
	Name      string          `json:"name,omitempty"`
	JSON      json.RawMessage `json:"json"`
}

type usageRecord struct {
	Provider    string       `json:"Provider"`
	AuthIndex   string       `json:"AuthIndex"`
	RequestedAt time.Time    `json:"RequestedAt"`
	Failed      bool         `json:"Failed"`
	Failure     usageFailure `json:"Failure"`
}

type usageFailure struct {
	StatusCode int    `json:"StatusCode"`
	Body       string `json:"Body"`
}

type lastRequestState struct {
	StatusCode int
	At         time.Time
}

var lastRequests = struct {
	sync.Mutex
	byAuthIndex map[string]lastRequestState
}{byAuthIndex: map[string]lastRequestState{}}

type accountRow struct {
	Name           string               `json:"name"`
	AuthIndex      string               `json:"auth_index"`
	Email          string               `json:"email"`
	Team           string               `json:"team"`
	Plan           string               `json:"plan"`
	AccountID      string               `json:"account_id"`
	Account        string               `json:"account"`
	Status         string               `json:"status"`
	StatusMessage  string               `json:"status_message"`
	Issue          string               `json:"issue"`
	Valid          bool                 `json:"valid"`
	Login          bool                 `json:"login"`
	Disabled       bool                 `json:"disabled"`
	Unavailable    bool                 `json:"unavailable"`
	LastRefresh    string               `json:"last_refresh"`
	NextRetryAfter string               `json:"next_retry_after"`
	Success        int64                `json:"success"`
	Failed         int64                `json:"failed"`
	RecentRequests []recentRequestEntry `json:"recent_requests"`
}

type accountsResponse struct {
	Accounts []accountRow `json:"accounts"`
}

type jwtClaims struct {
	Email string `json:"email"`
	Auth  struct {
		AccountID string `json:"chatgpt_account_id"`
		Plan      string `json:"chatgpt_plan_type"`
		Orgs      []struct {
			Title     string `json:"title"`
			Role      string `json:"role"`
			IsDefault bool   `json:"is_default"`
		} `json:"organizations"`
	} `json:"https://api.openai.com/auth"`
}

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if plugin == nil {
		return 1
	}
	C.store_host_api(host)
	plugin.abi_version = C.uint32_t(abiVersion)
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writeResponse(response, errorEnvelope("invalid_method", "method is required"))
		return 1
	}
	var requestBytes []byte
	if request != nil && requestLen > 0 {
		requestBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}
	raw, errHandle := handleMethod(C.GoString(method), requestBytes)
	if errHandle != nil {
		writeResponse(response, errorEnvelope("plugin_error", errHandle.Error()))
		return 1
	}
	writeResponse(response, raw)
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(ptr unsafe.Pointer, len C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
	_ = len
}

//export cliproxyPluginShutdown
func cliproxyPluginShutdown() {}

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case "plugin.register", "plugin.reconfigure":
		return okEnvelope(pluginRegistration())
	case "management.register":
		return okEnvelope(managementRegistration{
			Routes: []managementRoute{{
				Method:      http.MethodGet,
				Path:        "/plugins/" + pluginName + "/accounts",
				Description: "List Codex credentials with account metadata.",
			}},
			Resources: []managementResource{{
				Path:        "/index.html",
				Menu:        "Codex Credentials",
				Description: "Inspect Codex credentials and start OAuth login.",
			}},
		})
	case "management.handle":
		return handleManagement(request)
	case "usage.handle":
		if err := handleUsage(request); err != nil {
			return nil, err
		}
		return okEnvelope(map[string]any{})
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func pluginRegistration() registration {
	return registration{
		SchemaVersion: schemaVersion,
		Metadata: metadata{
			Name:             pluginName,
			Version:          pluginVersion,
			Author:           "Apocalypsor",
			GitHubRepository: "https://github.com/Apocalypsor/cpa-plugins",
			ConfigFields:     []configField{},
		},
		Capabilities: registrationCapabilities{ManagementAPI: true, UsagePlugin: true},
	}
}

func handleManagement(raw []byte) ([]byte, error) {
	var req managementRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	path := strings.TrimRight(req.Path, "/")
	if req.Method == http.MethodGet && isIndexResource(path) {
		return okEnvelope(managementResponse{
			StatusCode: http.StatusOK,
			Headers:    http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
			Body:       []byte(indexHTML),
		})
	}
	if req.Method == http.MethodGet && path == "/v0/management/plugins/"+pluginName+"/accounts" {
		rows, err := loadAccounts(strings.TrimSpace(req.Query.Get("auth_index")))
		if err != nil {
			return okEnvelope(jsonManagementResponse(http.StatusInternalServerError, map[string]any{"error": err.Error()}))
		}
		return okEnvelope(jsonManagementResponse(http.StatusOK, accountsResponse{Accounts: rows}))
	}
	return okEnvelope(jsonManagementResponse(http.StatusNotFound, map[string]any{"error": "not_found"}))
}

func isIndexResource(path string) bool {
	if path == "" {
		return true
	}
	path = strings.TrimSuffix(path, "/")
	return path == "/index.html" ||
		path == "/v0/resource/plugins/"+pluginName ||
		path == "/v0/resource/plugins/"+pluginName+"/index.html"
}

func loadAccounts(authIndex string) ([]accountRow, error) {
	list, err := callHostAuthList()
	if err != nil {
		return nil, err
	}
	rows := make([]accountRow, 0, len(list.Files))
	for _, entry := range list.Files {
		if !looksLikeCodex(entry) {
			continue
		}
		if authIndex != "" && entry.AuthIndex != authIndex {
			continue
		}
		row := rowFromEntry(entry)
		if !entry.RuntimeOnly && entry.AuthIndex != "" {
			if got, err := callHostAuthGet(entry.AuthIndex); err == nil {
				enrichRowFromJSON(&row, got.JSON)
				if got.Name != "" {
					row.Name = filepath.Base(got.Name)
				}
			}
		}
		row.Valid, row.Login, row.Issue = credentialState(row)
		applyLastRequestState(&row)
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		a := strings.ToLower(rows[i].Email + rows[i].Team + rows[i].Name)
		b := strings.ToLower(rows[j].Email + rows[j].Team + rows[j].Name)
		return a < b
	})
	return rows, nil
}

func handleUsage(raw []byte) error {
	var rec usageRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(rec.Provider)) != "codex" {
		return nil
	}
	authIndex := strings.TrimSpace(rec.AuthIndex)
	if authIndex == "" {
		return nil
	}
	status := http.StatusOK
	if rec.Failed {
		status = rec.Failure.StatusCode
		if status <= 0 {
			status = http.StatusInternalServerError
		}
	}
	at := rec.RequestedAt
	if at.IsZero() {
		at = time.Now()
	}
	lastRequests.Lock()
	if previous, ok := lastRequests.byAuthIndex[authIndex]; !ok || !previous.At.After(at) {
		lastRequests.byAuthIndex[authIndex] = lastRequestState{StatusCode: status, At: at}
	}
	lastRequests.Unlock()
	return nil
}

func applyLastRequestState(row *accountRow) {
	if row == nil || row.AuthIndex == "" {
		return
	}
	lastRequests.Lock()
	state, ok := lastRequests.byAuthIndex[row.AuthIndex]
	lastRequests.Unlock()
	if !ok || state.StatusCode != http.StatusUnauthorized {
		return
	}
	row.Valid = false
	row.Login = true
	row.Issue = "latest request HTTP 401"
}

func rowFromEntry(entry hostAuthFileEntry) accountRow {
	status := strings.TrimSpace(entry.Status)
	if status == "" {
		status = "unknown"
	}
	return accountRow{
		Name:           filepath.Base(entry.Name),
		AuthIndex:      entry.AuthIndex,
		Email:          entry.Email,
		Account:        strings.TrimSpace(entry.AccountType + " " + entry.Account),
		Status:         status,
		StatusMessage:  entry.StatusMessage,
		Disabled:       entry.Disabled,
		Unavailable:    entry.Unavailable,
		LastRefresh:    formatTime(entry.LastRefresh),
		NextRetryAfter: formatTime(entry.NextRetryAfter),
		Success:        entry.Success,
		Failed:         entry.Failed,
		RecentRequests: entry.RecentRequests,
	}
}

func enrichRowFromJSON(row *accountRow, raw json.RawMessage) {
	var rec map[string]any
	if err := json.Unmarshal(raw, &rec); err != nil {
		return
	}
	row.Email = firstNonEmpty(row.Email, stringField(rec, "email"))
	row.AccountID = firstNonEmpty(row.AccountID, stringField(rec, "account_id"))
	row.LastRefresh = firstNonEmpty(row.LastRefresh, stringField(rec, "last_refresh"))
	if claims, err := parseJWTClaims(stringField(rec, "id_token")); err == nil {
		row.Email = firstNonEmpty(row.Email, claims.Email)
		row.Plan = firstNonEmpty(row.Plan, strings.TrimSpace(claims.Auth.Plan))
		row.Team = firstNonEmpty(row.Team, defaultOrgTitle(claims))
		row.AccountID = firstNonEmpty(row.AccountID, strings.TrimSpace(claims.Auth.AccountID))
	}
	_, hasRefresh := rec["refresh_token"]
	if !hasRefresh || strings.TrimSpace(stringField(rec, "refresh_token")) == "" {
		row.Issue = "missing refresh token"
	}
}

func credentialState(row accountRow) (bool, bool, string) {
	for _, issue := range []string{row.Issue, row.StatusMessage, row.Status} {
		issue = strings.TrimSpace(issue)
		if requiresLoginIssue(issue) {
			return false, true, issue
		}
	}
	if row.Disabled {
		return false, false, firstNonEmpty(strings.TrimSpace(row.StatusMessage), "disabled")
	}
	if row.Unavailable {
		return false, false, firstNonEmpty(strings.TrimSpace(row.StatusMessage), strings.TrimSpace(row.Status), "unavailable")
	}
	status := strings.ToLower(strings.TrimSpace(row.Status))
	if status != "" && status != "active" {
		return false, false, firstNonEmpty(strings.TrimSpace(row.StatusMessage), strings.TrimSpace(row.Status))
	}
	return true, false, ""
}

func requiresLoginIssue(issue string) bool {
	raw := strings.ToLower(strings.TrimSpace(issue))
	if raw == "" {
		return false
	}
	for _, needle := range []string{
		"missing refresh token",
		"unauthorized",
		"status 401",
		"http 401",
		"401 unauthorized",
		"authentication_error",
		"authentication has expired",
		"log in again",
		"no_credentials",
		"invalid_grant",
		"invalid refresh token",
		"refresh_token_reused",
		"invalid credential",
		"invalid_credential",
	} {
		if strings.Contains(raw, needle) {
			return true
		}
	}
	return false
}

func looksLikeCodex(entry hostAuthFileEntry) bool {
	return entry.Provider == "codex" || entry.Type == "codex" ||
		(strings.HasPrefix(entry.Name, "codex-") && strings.HasSuffix(entry.Name, ".json"))
}

func parseJWTClaims(token string) (jwtClaims, error) {
	var claims jwtClaims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims, fmt.Errorf("invalid jwt")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, err
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return claims, err
	}
	return claims, nil
}

func defaultOrgTitle(claims jwtClaims) string {
	if len(claims.Auth.Orgs) == 0 {
		return ""
	}
	for _, org := range claims.Auth.Orgs {
		if org.IsDefault && strings.TrimSpace(org.Title) != "" {
			return strings.TrimSpace(org.Title)
		}
	}
	for _, org := range claims.Auth.Orgs {
		if strings.TrimSpace(org.Title) != "" {
			return strings.TrimSpace(org.Title)
		}
	}
	return ""
}

func callHostAuthList() (hostAuthListResponse, error) {
	result, err := callHost("host.auth.list", map[string]any{})
	if err != nil {
		return hostAuthListResponse{}, err
	}
	var resp hostAuthListResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return hostAuthListResponse{}, fmt.Errorf("decode host.auth.list: %w", err)
	}
	return resp, nil
}

func callHostAuthGet(authIndex string) (hostAuthGetResponse, error) {
	result, err := callHost("host.auth.get", hostAuthGetRequest{AuthIndex: authIndex})
	if err != nil {
		return hostAuthGetResponse{}, err
	}
	var resp hostAuthGetResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return hostAuthGetResponse{}, fmt.Errorf("decode host.auth.get: %w", err)
	}
	return resp, nil
}

func callHost(method string, payload any) (json.RawMessage, error) {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal host payload %s: %w", method, err)
	}
	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))

	var response C.cliproxy_buffer
	var requestPtr *C.uint8_t
	if len(rawPayload) > 0 {
		cPayload := C.CBytes(rawPayload)
		if cPayload == nil {
			return nil, fmt.Errorf("allocate host payload %s", method)
		}
		defer C.free(cPayload)
		requestPtr = (*C.uint8_t)(cPayload)
	}
	callCode := C.call_host_api(cMethod, requestPtr, C.size_t(len(rawPayload)), &response)
	var rawResponse []byte
	if response.ptr != nil && response.len > 0 {
		rawResponse = C.GoBytes(response.ptr, C.int(response.len))
	}
	if response.ptr != nil {
		C.free_host_buffer(response.ptr, response.len)
	}
	if len(rawResponse) == 0 {
		return nil, fmt.Errorf("host callback %s returned no response, code=%d", method, int(callCode))
	}
	var env envelope
	if err := json.Unmarshal(rawResponse, &env); err != nil {
		return nil, fmt.Errorf("decode host envelope %s: %w", method, err)
	}
	if !env.OK {
		if env.Error != nil {
			return nil, fmt.Errorf("%s: %s", env.Error.Code, env.Error.Message)
		}
		return nil, fmt.Errorf("host callback %s failed", method)
	}
	if callCode != 0 {
		return nil, fmt.Errorf("host callback %s returned code=%d", method, int(callCode))
	}
	return append(json.RawMessage(nil), env.Result...), nil
}

func jsonManagementResponse(status int, payload any) managementResponse {
	raw, err := json.Marshal(payload)
	if err != nil {
		status = http.StatusInternalServerError
		raw = []byte(`{"error":"failed to encode response"}`)
	}
	return managementResponse{
		StatusCode: status,
		Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
		Body:       raw,
	}
}

func okEnvelope(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{OK: true, Result: raw})
}

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}

func writeResponse(response *C.cliproxy_buffer, raw []byte) {
	if response == nil || len(raw) == 0 {
		return
	}
	ptr := C.CBytes(raw)
	if ptr == nil {
		return
	}
	response.ptr = ptr
	response.len = C.size_t(len(raw))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringField(rec map[string]any, key string) string {
	if rec == nil {
		return ""
	}
	switch v := rec[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

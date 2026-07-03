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
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

const (
	abiVersion      uint32 = 1
	schemaVersion   uint32 = 1
	pluginName             = "telegram-401-alert"
	pluginVersion          = "0.3.1"
	defaultCooldown        = 30 * time.Minute
)

var state = struct {
	sync.Mutex
	settings settings
	lastSent map[string]time.Time
}{
	settings: defaultSettings(),
	lastSent: map[string]time.Time{},
}

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type lifecycleRequest struct {
	ConfigYAML []byte `json:"config_yaml"`
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
	UsagePlugin   bool `json:"usage_plugin"`
	ManagementAPI bool `json:"management_api"`
}

type settings struct {
	BotToken string
	ChatID   string
	Cooldown time.Duration
}

type usageRecord struct {
	Provider    string        `json:"Provider"`
	Model       string        `json:"Model"`
	Alias       string        `json:"Alias"`
	AuthID      string        `json:"AuthID"`
	AuthIndex   string        `json:"AuthIndex"`
	AuthType    string        `json:"AuthType"`
	Source      string        `json:"Source"`
	RequestedAt time.Time     `json:"RequestedAt"`
	Latency     time.Duration `json:"Latency"`
	Failed      bool          `json:"Failed"`
	Failure     usageFailure  `json:"Failure"`
}

type usageFailure struct {
	StatusCode int    `json:"StatusCode"`
	Body       string `json:"Body"`
}

type httpRequest struct {
	Method  string      `json:"method"`
	URL     string      `json:"url"`
	Headers http.Header `json:"headers,omitempty"`
	Body    []byte      `json:"body,omitempty"`
}

type httpResponse struct {
	StatusCode int         `json:"StatusCode"`
	Headers    http.Header `json:"Headers,omitempty"`
	Body       []byte      `json:"Body,omitempty"`
}

type hostAuthGetRequest struct {
	AuthIndex string `json:"auth_index"`
}

type hostAuthRuntimeResponse struct {
	Auth hostAuthFileEntry `json:"auth"`
}

type hostAuthListResponse struct {
	Files []hostAuthFileEntry `json:"files"`
}

type hostAuthFileEntry struct {
	ID        string `json:"id,omitempty"`
	AuthIndex string `json:"auth_index,omitempty"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
}

type managementRegistration struct {
	Resources []managementResource `json:"Resources,omitempty"`
}

type managementResource struct {
	Path        string `json:"Path"`
	Menu        string `json:"Menu,omitempty"`
	Description string `json:"Description,omitempty"`
}

type managementRequest struct {
	Method string `json:"Method"`
	Path   string `json:"Path"`
}

type managementResponse struct {
	StatusCode int         `json:"StatusCode"`
	Headers    http.Header `json:"Headers,omitempty"`
	Body       []byte      `json:"Body,omitempty"`
}

const indexHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Telegram 401 Alert</title>
  <style>
    :root{color-scheme:light dark;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;--primary-color:#2563eb;--bg-primary:#fff;--bg-secondary:#f8fafc;--bg-tertiary:#eef2f7;--text-primary:#0f172a;--text-secondary:#475569;--border-color:#d9e2ec;--error-color:#dc2626;--radius-md:8px}
    body{margin:0;background:var(--bg-secondary);color:var(--text-primary)}
    main{max-width:760px;margin:0 auto;padding:32px 20px}
    h1{font-size:28px;font-weight:700;margin:0 0 24px}
    .card{background:var(--bg-primary);border:1px solid var(--border-color);border-radius:var(--radius-md);padding:18px;box-shadow:0 1px 2px rgba(15,23,42,.04)}
    .title{display:flex;align-items:center;gap:10px;font-size:18px;font-weight:700;margin-bottom:14px}
    .icon{width:24px;height:24px;border-radius:6px;background:linear-gradient(180deg,#38bdf8,#2563eb)}
    label{display:block;font-weight:600;margin:14px 0 6px}
    input{box-sizing:border-box;width:100%;font:inherit;padding:10px 12px;border:1px solid var(--border-color);border-radius:var(--radius-md);background:var(--bg-primary);color:var(--text-primary)}
    .row{display:flex;gap:10px;align-items:center;margin-top:16px;flex-wrap:wrap}
    button{font:inherit;font-weight:650;border:0;border-radius:var(--radius-md);padding:10px 14px;background:var(--primary-color);color:white;cursor:pointer}
    button.secondary{background:var(--bg-tertiary);color:var(--text-primary);border:1px solid var(--border-color)}
    button:disabled{opacity:.55;cursor:not-allowed}
    .status{padding:10px 12px;border-radius:var(--radius-md);font-size:14px;margin-top:14px;background:rgba(37,99,235,.12);color:var(--primary-color)}
    .status.success{background:rgba(34,197,94,.12);color:#16a34a}
    .status.error{background:rgba(220,38,38,.12);color:var(--error-color)}
    @media (prefers-color-scheme:dark){:root{--bg-primary:#111827;--bg-secondary:#0b1020;--bg-tertiary:#1f2937;--text-primary:#e5e7eb;--text-secondary:#a7b0bf;--border-color:#374151}}
    @media (max-width:520px){main{padding:20px 14px}}
  </style>
</head>
<body>
<main>
  <h1>Telegram 401 Alert</h1>
  <section class="card">
    <div class="title"><span class="icon" aria-hidden="true"></span><span>Telegram</span></div>
    <label for="key">CPA management key</label>
    <input id="key" type="password" autocomplete="off" placeholder="remote-management.secret-key">
    <label for="botToken">Bot token</label>
    <input id="botToken" type="password" autocomplete="off" placeholder="123456:ABC...">
    <label for="chatId">Chat ID</label>
    <input id="chatId" autocomplete="off" placeholder="123456789">
    <label for="cooldown">Cooldown seconds</label>
    <input id="cooldown" type="number" min="1" step="1" value="1800">
    <div class="row">
      <button id="save">Save</button>
      <button id="load" class="secondary">Reload</button>
    </div>
    <div id="status" class="status" hidden></div>
  </section>
</main>
<script>
const ENC_PREFIX = "enc::v1::";
const SECRET_SALT = "cli-proxy-api-webui::secure-storage";
const key = document.getElementById("key");
const botToken = document.getElementById("botToken");
const chatId = document.getElementById("chatId");
const cooldown = document.getElementById("cooldown");
const save = document.getElementById("save");
const load = document.getElementById("load");
const statusBox = document.getElementById("status");

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
  if (!res.ok) throw new Error(data.error || data.message || ("HTTP " + res.status));
  return data;
}

function setStatus(text, kind) {
  statusBox.hidden = false;
  statusBox.textContent = text;
  statusBox.className = "status" + (kind ? " " + kind : "");
}

async function loadConfig() {
  load.disabled = true;
  try {
    const data = await readJSON(await fetch("/v0/management/plugins/telegram-401-alert/config", {headers: headers(false)}));
    botToken.value = data.telegram_bot_token || "";
    chatId.value = data.telegram_chat_id || "";
    cooldown.value = data.cooldown_seconds || 1800;
    setStatus("Loaded.", "success");
  } catch (err) {
    if (err.message !== "missing key") setStatus(String(err), "error");
  } finally {
    load.disabled = false;
  }
}

async function saveConfig() {
  save.disabled = true;
  try {
    const seconds = Math.max(1, Number.parseInt(cooldown.value || "1800", 10));
    await readJSON(await fetch("/v0/management/plugins/telegram-401-alert/config", {
      method: "PATCH",
      headers: headers(true),
      body: JSON.stringify({
        telegram_bot_token: botToken.value.trim(),
        telegram_chat_id: chatId.value.trim(),
        cooldown_seconds: seconds
      })
    }));
    cooldown.value = seconds;
    setStatus("Saved. CPA will reload the plugin config.", "success");
  } catch (err) {
    if (err.message !== "missing key") setStatus(String(err), "error");
  } finally {
    save.disabled = false;
  }
}

key.value = readSavedManagementKey();
load.addEventListener("click", loadConfig);
save.addEventListener("click", saveConfig);
if (key.value) loadConfig();
</script>
</body>
</html>`

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
		applyConfig(request)
		return okEnvelope(pluginRegistration())
	case "usage.handle":
		if err := handleUsage(request); err != nil {
			logHost("error", "telegram-401-alert failed", map[string]any{"error": err.Error()})
		}
		return okEnvelope(map[string]any{})
	case "management.register":
		return okEnvelope(managementRegistration{Resources: []managementResource{{
			Path:        "/index.html",
			Menu:        "Telegram 401 Alert",
			Description: "Configure Telegram notifications for account 401 failures.",
		}}})
	case "management.handle":
		return handleManagement(request)
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
			ConfigFields: []configField{
				{Name: "telegram_bot_token", Type: "string", Description: "Telegram bot token from BotFather."},
				{Name: "telegram_chat_id", Type: "string", Description: "Telegram target chat id."},
				{Name: "cooldown_seconds", Type: "integer", Description: "Duplicate alert cooldown per account. Default: 1800."},
			},
		},
		Capabilities: registrationCapabilities{UsagePlugin: true, ManagementAPI: true},
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
	return okEnvelope(managementResponse{
		StatusCode: http.StatusNotFound,
		Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
		Body:       []byte(`{"error":"not_found"}`),
	})
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

func applyConfig(raw []byte) {
	var req lifecycleRequest
	_ = json.Unmarshal(raw, &req)
	next := parseSettings(req.ConfigYAML)
	state.Lock()
	state.settings = next
	state.Unlock()
}

func handleUsage(raw []byte) error {
	var rec usageRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return err
	}
	if !rec.Failed || rec.Failure.StatusCode != http.StatusUnauthorized {
		return nil
	}

	cfg := currentSettings()
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return nil
	}

	email := resolveAuthEmail(rec)
	key, once := notificationKey(rec, email)
	now := time.Now()
	if !reserveNotification(key, now, cfg.Cooldown, once) {
		return nil
	}
	if err := sendTelegram(cfg, telegramMessage(rec, email)); err != nil {
		forgetReservation(key, now)
		return err
	}
	return nil
}

func currentSettings() settings {
	state.Lock()
	defer state.Unlock()
	return state.settings
}

func reserveNotification(key string, now time.Time, cooldown time.Duration, once bool) bool {
	state.Lock()
	defer state.Unlock()
	if last, ok := state.lastSent[key]; ok {
		if once || now.Sub(last) < cooldown {
			return false
		}
	}
	state.lastSent[key] = now
	return true
}

func forgetReservation(key string, at time.Time) {
	state.Lock()
	defer state.Unlock()
	if state.lastSent[key].Equal(at) {
		delete(state.lastSent, key)
	}
}

func sendTelegram(cfg settings, text string) error {
	body, err := json.Marshal(map[string]any{
		"chat_id":                  cfg.ChatID,
		"text":                     text,
		"disable_web_page_preview": true,
	})
	if err != nil {
		return err
	}
	respRaw, err := callHost("host.http.do", httpRequest{
		Method: http.MethodPost,
		URL:    "https://api.telegram.org/bot" + cfg.BotToken + "/sendMessage",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
			"User-Agent":   []string{pluginName + "/" + pluginVersion},
		},
		Body: body,
	})
	if err != nil {
		return err
	}
	var resp httpResponse
	if err := json.Unmarshal(respRaw, &resp); err != nil {
		return fmt.Errorf("decode telegram response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram returned HTTP %d: %s", resp.StatusCode, trimForMessage(string(resp.Body), 300))
	}
	return nil
}

func telegramMessage(rec usageRecord, email string) string {
	var b strings.Builder
	b.WriteString("CPA account 401\n")
	writeLine(&b, "Provider", rec.Provider)
	writeLine(&b, "Email", email)
	writeLine(&b, "Auth", firstNonEmpty(rec.AuthIndex, rec.AuthID, rec.AuthType))
	writeLine(&b, "Model", firstNonEmpty(rec.Alias, rec.Model))
	writeLine(&b, "Source", rec.Source)
	if !rec.RequestedAt.IsZero() {
		writeLine(&b, "Time", rec.RequestedAt.Format(time.RFC3339))
	}
	if rec.Failure.Body != "" {
		writeLine(&b, "Error", trimForMessage(rec.Failure.Body, 500))
	}
	return strings.TrimSpace(b.String())
}

func writeLine(b *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%s: %s\n", key, value)
}

func notificationKey(rec usageRecord, email string) (string, bool) {
	if email = strings.ToLower(strings.TrimSpace(email)); email != "" {
		return "email:" + email, true
	}
	parts := []string{rec.Provider, rec.AuthIndex, rec.AuthID}
	if strings.TrimSpace(rec.AuthIndex) == "" && strings.TrimSpace(rec.AuthID) == "" {
		parts = []string{rec.Provider, rec.Model}
	}
	return "account:" + strings.Join(parts, "/"), false
}

func resolveAuthEmail(rec usageRecord) string {
	if rec.AuthIndex != "" {
		if email, err := emailFromRuntimeAuth(rec.AuthIndex); err == nil && email != "" {
			return email
		}
	}
	if email, err := emailFromAuthList(rec); err == nil {
		return email
	}
	return ""
}

func emailFromRuntimeAuth(authIndex string) (string, error) {
	result, err := callHost("host.auth.get_runtime", hostAuthGetRequest{AuthIndex: strings.TrimSpace(authIndex)})
	if err != nil {
		return "", err
	}
	var resp hostAuthRuntimeResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Auth.Email), nil
}

func emailFromAuthList(rec usageRecord) (string, error) {
	result, err := callHost("host.auth.list", map[string]any{})
	if err != nil {
		return "", err
	}
	var resp hostAuthListResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}
	authIndex := strings.TrimSpace(rec.AuthIndex)
	authID := strings.TrimSpace(rec.AuthID)
	for _, file := range resp.Files {
		if authIndex != "" && strings.TrimSpace(file.AuthIndex) == authIndex {
			return strings.TrimSpace(file.Email), nil
		}
		if authID != "" && strings.TrimSpace(file.ID) == authID {
			return strings.TrimSpace(file.Email), nil
		}
		if authIndex != "" && strings.TrimSpace(file.Name) == authIndex {
			return strings.TrimSpace(file.Email), nil
		}
	}
	return "", nil
}

func defaultSettings() settings {
	return settings{Cooldown: defaultCooldown}
}

func parseSettings(raw []byte) settings {
	cfg := defaultSettings()
	values := parseYAMLScalars(raw)
	cfg.BotToken = strings.TrimSpace(values["telegram_bot_token"])
	cfg.ChatID = strings.TrimSpace(values["telegram_chat_id"])
	if seconds, ok := parsePositiveInt(values["cooldown_seconds"]); ok {
		cfg.Cooldown = time.Duration(seconds) * time.Second
	}
	return cfg
}

func parseYAMLScalars(raw []byte) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(stripYAMLComment(line))
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = unquoteYAMLScalar(strings.TrimSpace(value))
	}
	return out
}

func stripYAMLComment(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "#") {
		return ""
	}
	if i := strings.Index(line, " #"); i >= 0 {
		return line[:i]
	}
	return line
}

func unquoteYAMLScalar(value string) string {
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
	}
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'")
	}
	return value
}

func parsePositiveInt(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	n, err := strconv.Atoi(value)
	return n, err == nil && n > 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func trimForMessage(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
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

func logHost(level, message string, fields map[string]any) {
	_, _ = callHost("host.log", map[string]any{
		"level":   level,
		"message": message,
		"fields":  fields,
	})
}

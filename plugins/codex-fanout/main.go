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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
	"unsafe"
)

const (
	abiVersion    uint32 = 1
	schemaVersion uint32 = 1
	pluginName           = "codex-fanout"
	pluginVersion        = "0.1.0"
)

var copyFields = []string{"access_token", "id_token", "expired", "last_refresh"}

const indexHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Codex Fan-out</title>
  <style>
    :root{color-scheme:light dark;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
    body{margin:0;background:#f6f7f9;color:#111827}
    main{max-width:860px;margin:0 auto;padding:32px 20px}
    h1{font-size:24px;margin:0 0 20px}
    label{display:block;font-weight:600;margin:18px 0 6px}
    input{box-sizing:border-box;width:100%;font:inherit;padding:10px 12px;border:1px solid #c7ccd3;border-radius:8px;background:white;color:#111827}
    .row{display:flex;gap:10px;align-items:center;margin-top:16px;flex-wrap:wrap}
    button{font:inherit;font-weight:650;border:0;border-radius:8px;padding:10px 14px;background:#0f766e;color:white;cursor:pointer}
    button.secondary{background:#374151}
    button:disabled{opacity:.55;cursor:not-allowed}
    .check{display:flex;gap:8px;align-items:center;font-weight:500}
    .check input{width:auto}
    pre{margin-top:18px;padding:14px;min-height:260px;white-space:pre-wrap;overflow:auto;border-radius:8px;background:#111827;color:#e5e7eb;font:13px ui-monospace,SFMono-Regular,Menlo,monospace}
    .hint{color:#4b5563;font-size:13px;margin-top:8px}
    @media (prefers-color-scheme:dark){body{background:#0b1020;color:#e5e7eb}input{background:#111827;color:#e5e7eb;border-color:#374151}.hint{color:#9ca3af}}
  </style>
</head>
<body>
<main>
  <h1>Codex Fan-out</h1>
  <label for="key">CPA management key</label>
  <input id="key" type="password" autocomplete="off" placeholder="remote-management.secret-key">
  <div class="hint">The key stays in this tab only. The plugin uses CPA host auth callbacks server-side.</div>

  <label for="master">Optional master filenames</label>
  <input id="master" placeholder="codex-...json, codex-...json">

  <div class="row">
    <label class="check"><input id="noBackup" type="checkbox"> skip .bak files</label>
  </div>
  <div class="row">
    <button id="dry">Dry run</button>
    <button id="apply" class="secondary">Apply</button>
  </div>
  <pre id="out">Ready.</pre>
</main>
<script>
const out = document.getElementById("out");
const key = document.getElementById("key");
const master = document.getElementById("master");
const noBackup = document.getElementById("noBackup");
const dry = document.getElementById("dry");
const apply = document.getElementById("apply");

async function run(dryRun) {
  const token = key.value.trim();
  if (!token) {
    out.textContent = "Missing CPA management key.";
    key.focus();
    return;
  }
  dry.disabled = true;
  apply.disabled = true;
  out.textContent = dryRun ? "Running dry-run..." : "Applying fan-out...";
  try {
    const res = await fetch("/v0/management/plugins/codex-fanout/fanout", {
      method: "POST",
      headers: {
        "Authorization": "Bearer " + token,
        "Content-Type": "application/json"
      },
      body: JSON.stringify({
        dry_run: dryRun,
        no_backup: noBackup.checked,
        master: master.value.trim()
      })
    });
    const text = await res.text();
    let data;
    try { data = JSON.parse(text); } catch { data = { output: text }; }
    out.textContent = data.output || text || ("HTTP " + res.status);
  } catch (err) {
    out.textContent = String(err);
  } finally {
    dry.disabled = false;
    apply.disabled = false;
  }
}

dry.addEventListener("click", () => run(true));
apply.addEventListener("click", () => run(false));
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
	CommandLinePlugin bool `json:"command_line_plugin"`
	ManagementAPI     bool `json:"management_api"`
}

type commandLineRegistration struct {
	Flags []commandLineFlag `json:"Flags"`
}

type commandLineFlag struct {
	Name         string `json:"Name"`
	Usage        string `json:"Usage"`
	Type         string `json:"Type"`
	DefaultValue string `json:"DefaultValue,omitempty"`
}

type commandLineRequest struct {
	Flags          map[string]commandLineFlagValue `json:"Flags"`
	TriggeredFlags map[string]commandLineFlagValue `json:"TriggeredFlags"`
	Host           hostConfigSummary               `json:"Host"`
}

type commandLineFlagValue struct {
	Value string `json:"Value"`
	Set   bool   `json:"Set"`
}

type hostConfigSummary struct {
	AuthDir string `json:"AuthDir"`
}

type commandLineResponse struct {
	Stdout   []byte `json:"Stdout,omitempty"`
	Stderr   []byte `json:"Stderr,omitempty"`
	ExitCode int    `json:"ExitCode,omitempty"`
}

type managementRegistration struct {
	Routes    []managementRoute    `json:"Routes"`
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
	Method  string      `json:"Method"`
	Path    string      `json:"Path"`
	Headers http.Header `json:"Headers"`
	Query   url.Values  `json:"Query"`
	Body    []byte      `json:"Body"`
}

type managementResponse struct {
	StatusCode int         `json:"StatusCode,omitempty"`
	Headers    http.Header `json:"Headers,omitempty"`
	Body       []byte      `json:"Body,omitempty"`
}

type fanoutRequest struct {
	DryRun   bool   `json:"dry_run"`
	NoBackup bool   `json:"no_backup"`
	Master   string `json:"master"`
}

type hostAuthFileEntry struct {
	AuthIndex   string    `json:"auth_index,omitempty"`
	Name        string    `json:"name"`
	Type        string    `json:"type,omitempty"`
	Provider    string    `json:"provider,omitempty"`
	RuntimeOnly bool      `json:"runtime_only,omitempty"`
	LastRefresh time.Time `json:"last_refresh,omitempty"`
	Email       string    `json:"email,omitempty"`
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
	Path      string          `json:"path,omitempty"`
	JSON      json.RawMessage `json:"json"`
}

type hostAuthSaveRequest struct {
	Name string          `json:"name"`
	JSON json.RawMessage `json:"json"`
}

type fanoutOptions struct {
	DryRun  bool
	Backup  bool
	Masters map[string]bool
}

type authFile struct {
	Index string
	Name  string
	Path  string
	Rec   map[string]any
	Raw   json.RawMessage
}

type updatePlan struct {
	Target  *authFile
	Changed map[string]any
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
	raw, err := handleMethod(C.GoString(method), requestBytes)
	if err != nil {
		writeResponse(response, errorEnvelope("plugin_error", err.Error()))
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
	case "command_line.register":
		return okEnvelope(commandLineRegistration{Flags: []commandLineFlag{
			{Name: pluginName, Usage: "Fan out one Codex access token to sibling workspace auth files", Type: "bool"},
			{Name: pluginName + "-dry-run", Usage: "Preview Codex fan-out changes without writing", Type: "bool"},
			{Name: pluginName + "-no-backup", Usage: "Do not write .bak files before updating auth files", Type: "bool"},
			{Name: pluginName + "-master", Usage: "Comma-separated master auth filenames; optional", Type: "string"},
		}})
	case "command_line.execute":
		return handleCommandLine(request)
	case "management.register":
		return okEnvelope(managementRegistration{
			Routes: []managementRoute{
				{
					Method:      http.MethodPost,
					Path:        "/plugins/" + pluginName + "/fanout",
					Description: "Run Codex token fan-out. Body: dry_run, no_backup, master.",
				},
			},
			Resources: []managementResource{
				{
					Path:        "/index.html",
					Menu:        "Codex Fan-out",
					Description: "Web UI for previewing and applying Codex auth fan-out.",
				},
			},
		})
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
			Author:           "local",
			GitHubRepository: "https://github.com/router-for-me/CLIProxyAPI",
			ConfigFields:     []configField{},
		},
		Capabilities: registrationCapabilities{CommandLinePlugin: true, ManagementAPI: true},
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
	if req.Method == http.MethodPost && path == "/v0/management/plugins/"+pluginName+"/fanout" {
		return okEnvelope(handleFanoutAPI(req.Body))
	}
	return okEnvelope(jsonManagementResponse(http.StatusNotFound, map[string]any{
		"error": "not_found",
	}))
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

func handleFanoutAPI(body []byte) managementResponse {
	var req fanoutRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{
				"ok":     false,
				"output": "invalid JSON: " + err.Error(),
			})
		}
	}
	opts := fanoutOptions{
		DryRun:  req.DryRun,
		Backup:  !req.NoBackup,
		Masters: parseMasters(req.Master),
	}
	var out bytes.Buffer
	if err := runFanout(opts, &out); err != nil {
		fmt.Fprintf(&out, "error: %v\n", err)
		return jsonManagementResponse(http.StatusInternalServerError, map[string]any{
			"ok":     false,
			"output": out.String(),
		})
	}
	return jsonManagementResponse(http.StatusOK, map[string]any{
		"ok":     true,
		"output": out.String(),
	})
}

func handleCommandLine(raw []byte) ([]byte, error) {
	var req commandLineRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("decode command request: %w", err)
		}
	}
	if !flagBool(req.Flags, pluginName) {
		return okEnvelope(commandLineResponse{Stderr: []byte("use -" + pluginName + " to run codex fan-out\n"), ExitCode: 2})
	}
	opts := fanoutOptions{
		DryRun:  flagBool(req.Flags, pluginName+"-dry-run"),
		Backup:  !flagBool(req.Flags, pluginName+"-no-backup"),
		Masters: parseMasters(flagString(req.Flags, pluginName+"-master")),
	}
	var out bytes.Buffer
	if req.Host.AuthDir != "" {
		fmt.Fprintf(&out, "auth-dir: %s\n", req.Host.AuthDir)
	}
	code := 0
	if err := runFanout(opts, &out); err != nil {
		fmt.Fprintf(&out, "error: %v\n", err)
		code = 1
	}
	return okEnvelope(commandLineResponse{Stdout: out.Bytes(), ExitCode: code})
}

func jsonManagementResponse(status int, payload any) managementResponse {
	raw, err := json.Marshal(payload)
	if err != nil {
		status = http.StatusInternalServerError
		raw = []byte(`{"ok":false,"output":"failed to encode response"}`)
	}
	return managementResponse{
		StatusCode: status,
		Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
		Body:       raw,
	}
}

func runFanout(opts fanoutOptions, out *bytes.Buffer) error {
	auths, err := loadCodexAuths()
	if err != nil {
		return err
	}
	if len(auths) == 0 {
		fmt.Fprintln(out, "no codex auth files found")
		return nil
	}
	groups := groupByEmail(auths)
	total := 0
	for email, members := range groups {
		master := chooseMaster(members, opts.Masters)
		fmt.Fprintf(out, "\n[%s] master = %s\n", email, master.Name)
		fmt.Fprintf(out, "    last_refresh=%v expired=%v\n", master.Rec["last_refresh"], master.Rec["expired"])
		if stringField(master.Rec, "refresh_token") == "" {
			fmt.Fprintln(out, "    skip: master has no refresh_token")
			continue
		}
		if stringField(master.Rec, "access_token") == "" {
			fmt.Fprintln(out, "    skip: master has no access_token")
			continue
		}
		plans := planUpdates(master, members)
		for _, plan := range plans {
			names := sortedKeys(plan.Changed)
			if len(names) == 0 {
				fmt.Fprintf(out, "    = %s (already synced)\n", plan.Target.Name)
				continue
			}
			if opts.DryRun {
				fmt.Fprintf(out, "    ~ %s would update: %s (account_id keeps %v)\n", plan.Target.Name, strings.Join(names, ","), plan.Target.Rec["account_id"])
				continue
			}
			if opts.Backup {
				if err := backupAuth(plan.Target); err != nil {
					return fmt.Errorf("backup %s: %w", plan.Target.Name, err)
				}
			}
			next := cloneMap(plan.Target.Rec)
			for k, v := range plan.Changed {
				next[k] = v
			}
			raw, err := json.MarshalIndent(next, "", "  ")
			if err != nil {
				return fmt.Errorf("encode %s: %w", plan.Target.Name, err)
			}
			raw = append(raw, '\n')
			if err := callHostAuthSave(plan.Target.Name, raw); err != nil {
				return fmt.Errorf("save %s: %w", plan.Target.Name, err)
			}
			total++
			fmt.Fprintf(out, "    + %s updated: %s\n", plan.Target.Name, strings.Join(names, ","))
		}
	}
	if opts.DryRun {
		fmt.Fprintln(out, "\ndone (dry-run, no files changed)")
	} else {
		fmt.Fprintf(out, "\ndone. updated %d sibling files\n", total)
	}
	return nil
}

func loadCodexAuths() ([]*authFile, error) {
	list, err := callHostAuthList()
	if err != nil {
		return nil, err
	}
	var auths []*authFile
	for _, f := range list.Files {
		if f.RuntimeOnly || f.AuthIndex == "" || !looksLikeCodex(f) {
			continue
		}
		got, err := callHostAuthGet(f.AuthIndex)
		if err != nil {
			return nil, fmt.Errorf("get %s: %w", f.Name, err)
		}
		var rec map[string]any
		if err := json.Unmarshal(got.JSON, &rec); err != nil {
			return nil, fmt.Errorf("decode %s: %w", f.Name, err)
		}
		if rec["type"] != "codex" {
			continue
		}
		name := got.Name
		if name == "" {
			name = f.Name
		}
		auths = append(auths, &authFile{
			Index: f.AuthIndex,
			Name:  filepath.Base(name),
			Path:  got.Path,
			Rec:   rec,
			Raw:   append(json.RawMessage(nil), got.JSON...),
		})
	}
	sort.Slice(auths, func(i, j int) bool { return auths[i].Name < auths[j].Name })
	return auths, nil
}

func groupByEmail(auths []*authFile) map[string][]*authFile {
	groups := map[string][]*authFile{}
	for _, a := range auths {
		email := stringField(a.Rec, "email")
		if email == "" {
			continue
		}
		groups[email] = append(groups[email], a)
	}
	return groups
}

func chooseMaster(members []*authFile, manual map[string]bool) *authFile {
	for _, a := range members {
		if manual[a.Name] {
			return a
		}
	}
	var pool []*authFile
	for _, a := range members {
		if stringField(a.Rec, "refresh_token") != "" {
			pool = append(pool, a)
		}
	}
	if len(pool) == 0 {
		pool = members
	}
	best := pool[0]
	for _, a := range pool[1:] {
		if parseRefreshTime(a).After(parseRefreshTime(best)) {
			best = a
		}
	}
	return best
}

func planUpdates(master *authFile, members []*authFile) []updatePlan {
	plans := make([]updatePlan, 0, len(members)-1)
	for _, target := range members {
		if target == master {
			continue
		}
		changed := map[string]any{}
		for _, field := range copyFields {
			if !reflect.DeepEqual(target.Rec[field], master.Rec[field]) {
				changed[field] = master.Rec[field]
			}
		}
		if stringField(target.Rec, "refresh_token") != "" {
			changed["refresh_token"] = ""
		}
		plans = append(plans, updatePlan{Target: target, Changed: changed})
	}
	return plans
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

func callHostAuthSave(name string, raw json.RawMessage) error {
	_, err := callHost("host.auth.save", hostAuthSaveRequest{Name: name, JSON: raw})
	return err
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

func backupAuth(a *authFile) error {
	if a.Path == "" {
		return fmt.Errorf("host did not return a physical path")
	}
	raw, err := os.ReadFile(a.Path)
	if err != nil {
		raw = a.Raw
	}
	mode := os.FileMode(0600)
	if st, statErr := os.Stat(a.Path); statErr == nil {
		mode = st.Mode().Perm()
	}
	return os.WriteFile(a.Path+".bak", raw, mode)
}

func looksLikeCodex(f hostAuthFileEntry) bool {
	if f.Type == "codex" || f.Provider == "codex" {
		return true
	}
	return strings.HasPrefix(f.Name, "codex-") && strings.HasSuffix(f.Name, ".json")
}

func parseRefreshTime(a *authFile) time.Time {
	return parseTime(stringField(a.Rec, "last_refresh"))
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	layouts := []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func flagBool(flags map[string]commandLineFlagValue, name string) bool {
	v, ok := flags[name]
	if !ok || !v.Set {
		return false
	}
	return strings.EqualFold(v.Value, "true") || v.Value == "1"
}

func flagString(flags map[string]commandLineFlagValue, name string) string {
	v, ok := flags[name]
	if !ok || !v.Set {
		return ""
	}
	return strings.TrimSpace(v.Value)
}

func parseMasters(raw string) map[string]bool {
	masters := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			masters[filepath.Base(item)] = true
		}
	}
	return masters
}

func stringField(rec map[string]any, key string) string {
	v, _ := rec[key].(string)
	return v
}

func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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

var _ = http.Header{}
var _ = url.Values{}

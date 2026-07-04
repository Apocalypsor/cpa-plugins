# CPA Plugins

Personal CLIProxyAPI plugin registry and plugin sources.

## Layout

```text
registry/registry.json        # CPA plugin source
plugins/<plugin-id>/          # one dynamic-library plugin per directory
.github/workflows/release.yml # builds release zips for every plugin
```

Current plugins:

- `codex-credentials`: show Codex credential email, team, plan, status, and
  OAuth login actions at `/v0/resource/plugins/codex-credentials/index.html`.
- `telegram-401-alert`: send a Telegram notification when CPA reports an account
  HTTP 401 failure. Configure it from CPA's built-in plugin config UI.

## Use In CLIProxyAPI

Add this source to `config.yaml`:

```yaml
plugins:
  enabled: true
  dir: "/CLIProxyAPI/plugins"
  store-sources:
    - "https://raw.githubusercontent.com/Apocalypsor/cpa-plugins/main/registry/registry.json"
```

For Docker, mount the plugin directory so installed plugins survive container
updates:

```yaml
services:
  cli-proxy-api:
    volumes:
      - ./config.yaml:/CLIProxyAPI/config.yaml
      - ./auths:/root/.cli-proxy-api
      - ./plugins:/CLIProxyAPI/plugins
```

## Publish

Releases are repo-wide. A tag like `v0.1.0` builds these assets for every plugin:

```text
<plugin-id>_0.1.0_linux_amd64.zip
<plugin-id>_0.1.0_linux_arm64.zip
checksums.txt
```

Each zip contains exactly one dynamic library at the zip root:

```text
codex-credentials.so
```

To add another plugin:

1. Create `plugins/<plugin-id>/` with its Go plugin source.
2. Add one entry to `registry/registry.json`.
3. Tag a new repo release.

Skipped: per-plugin versioning. Add it when two plugins need independent release cadence.

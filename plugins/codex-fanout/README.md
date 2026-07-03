# codex-fanout

CLIProxyAPI command-line plugin that fans out one Codex access token to sibling
workspace auth files for the same email.

Flags:

- `-codex-fanout`: run the command.
- `-codex-fanout-dry-run`: preview changes.
- `-codex-fanout-no-backup`: skip `.bak` files.
- `-codex-fanout-master`: comma-separated master auth filenames.

Run:

```bash
cli-proxy-api -config /path/to/config.yaml -codex-fanout -codex-fanout-dry-run
cli-proxy-api -config /path/to/config.yaml -codex-fanout
```

Web UI:

```text
http://<cpa-host>:<api-port>/v0/resource/plugins/codex-fanout/index.html
```

The page is a static plugin resource. Enter CPA's management key, then use
`Dry run` or `Apply`. The browser never reads auth JSON; the plugin calls CPA's
server-side `host.auth.list`, `host.auth.get`, and `host.auth.save` callbacks.

Build locally:

```bash
make test
make build
```

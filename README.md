# netfu

A vim-friendly TUI replacement for nmtui. Full NetworkManager control — live
wifi scanning, devices, connection editing, hostname, VPN activation — in a
single pane with four tabs. Talks to NetworkManager over D-Bus; no nmcli.

## Build

```sh
go build ./cmd/netfu
./netfu          # against the running NetworkManager
./netfu --fake   # demo mode with seeded fixture data, no NM needed
./netfu --version
```

## Tabs

1. **Wi-Fi** — live scan list (deduped per SSID, signal-sorted), join with
   password modal, wrong-password retry, hidden networks, out-of-range saved
   section.
2. **Devices** — managed devices with state; activate/deactivate, detail view.
3. **Connections** — every saved profile grouped by type; typed editor for
   the fields you actually change, untouched settings preserved verbatim.
4. **System** — hostname, wifi radio, NetworkManager state, VPN
   activate/deactivate (D-Bus cannot create VPN profiles).

## Key bindings

| Key | Action |
| --- | --- |
| `1`–`4`, `[` / `]` | switch tab |
| `j` / `k`, `g` / `G` | move / jump to top, bottom |
| `↵` | contextual, never destructive: connect, edit, confirm-deactivate |
| `/` | filter (wifi, devices) |
| `?` | help |
| `W` | wifi radio toggle (global) |
| `q` | pop layer; quit at top level (`ctrl+c` always quits) |
| `esc` | pop layer / close modal / clear filter — never quits |

Per tab: Wi-Fi `c` join hidden, `d` disconnect · Devices `i` detail,
`a`/`d` activate/deactivate · Connections `e` edit, `n` new, `x` delete,
`a`/`d` · System `i`/`↵` edit field, `space` toggle, `a`/`d` VPN.

Editor: `j`/`k` fields, `↵`/`i` edit field, `space` cycle, `s` save,
`esc`/`q` back.

## Testing

```sh
go test ./...                                      # full suite, no NM needed
go test -tags nmintegration ./internal/backend/nm/ # read-only, against real NM
```

Developed and integration-tested against NetworkManager 1.56.1 with pinned
gonetworkmanager v3.2.0 and bubbletea v2.0.8 (see go.mod).

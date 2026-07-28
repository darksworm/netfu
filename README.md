# netfu

A vim-friendly TUI replacement for nmtui. Full NetworkManager control — live
wifi scanning, devices, connection editing, hostname, VPN activation — one
tab per physical interface plus Virtual, Other, Auto and System. Talks to
NetworkManager over D-Bus; no nmcli.

## Build

```sh
go build ./cmd/netfu
./netfu          # against the running NetworkManager
./netfu --fake   # demo mode with seeded fixture data, no NM needed
./netfu --version
```

## Tabs

The first tabs are the machine's physical interfaces (wifi first, then
ethernet, labeled by interface name); Virtual, Other, Auto and System
always close the bar. Device hotplug re-derives the tabs live.

1. **Wifi device** (e.g. `wlan0`) — the home tab: live scan list (deduped
   per SSID, signal-sorted), join with password modal, wrong-password
   retry, hidden networks, out-of-range saved section, and wifi profile
   management (`e` edit, `x` forget).
2. **Ethernet device** (e.g. `enp0s31f6`) — device detail plus the wired
   profiles usable on this NIC (unpinned or pinned to it), most recently
   used first; `a`/`↵` activates the selected profile, `d` deactivates
   with confirm, `e` edit, `x` delete, `n` new profile pinned to this NIC.
3. **Virtual** — bridges, veth, tun & co with state; activate/deactivate,
   detail view (`p2p-dev-*` noise is hidden).
4. **Other** — saved profiles without their own tab: VPN, bridge, bond,
   vlan, plus orphaned wired profiles (no NIC present, or pinned to an
   interface no current device has); typed editor for the fields you
   actually change, untouched settings preserved verbatim; VPN
   activate/deactivate lives here (D-Bus cannot create VPN profiles).
5. **Auto** — the autoconnect pick order (priority desc, then most
   recently used, sectioned per device type) without thinking in numbers:
   `J`/`K` reorder, `space` toggles autoconnect, `s` writes spaced
   priorities back — only to profiles that actually changed.
6. **System** — hostname, wifi radio, NetworkManager state.

## Key bindings

| Key | Action |
| --- | --- |
| `1`–`9`, `[` / `]` | switch tab |
| `j` / `k`, `g` / `G` | move / jump to top, bottom |
| `↵` | contextual, never destructive: connect, activate, edit, confirm-deactivate |
| `/` | filter (wifi, virtual) |
| `?` | help |
| `W` | wifi radio toggle (global) |
| `q` | pop layer; quit at top level (`ctrl+c` always quits) |
| `esc` | pop layer / close modal / clear filter — never quits |

Per tab: Wifi `c` join hidden, `d` disconnect, `e` edit, `x` forget ·
Ethernet `a`/`↵` activate, `d` deactivate, `e` edit, `x` delete, `n` new ·
Virtual `i` detail, `a`/`d` · Other `e`/`↵` edit, `n` new, `x` delete,
`a`/`d` · Auto `J`/`K` reorder, `space` toggle, `s` save · System `i`/`↵`
edit field, `space` toggle.

Editor: `j`/`k` fields, `↵`/`i` edit field, `space` cycle, `s` save,
`esc`/`q` back.

## Testing

```sh
go test ./...                                      # full suite, no NM needed
go test -tags nmintegration ./internal/backend/nm/ # read-only, against real NM
```

Developed and integration-tested against NetworkManager 1.56.1 with pinned
gonetworkmanager v3.2.0 and bubbletea v2.0.8 (see go.mod).

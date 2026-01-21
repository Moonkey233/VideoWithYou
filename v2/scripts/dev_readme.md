# Dev Notes (VideoWithYou v2)

## Quick Start (Windows)

1) Build server + client (from `v2/`):

```
.\scripts\build.ps1
```

2) Run the server:

```
./bin/server.exe
```

3) Build the extension:

```
cd extension
npm install
npm run build
```

4) Load the extension in Edge/Chrome:
- Extensions -> Load unpacked -> select `v2/extension/dist`

5) Run local client manually (keeps running):

```
./bin/local-client.exe
```

Open the popup in the browser to create/join rooms. The extension connects to
`ws://127.0.0.1:27111/ext` by default (configurable via the popup `Client Port`).

## Config

`v2/local-client/config.json` controls:
- `ext_listen_addr` / `ext_listen_path`: extension bridge endpoint
- `ext_idle_timeout_sec`: browser endpoint idle window (0 disables)
- `endpoint_inactive_timeout_sec`: follower leave timeout after endpoint missing (0 disables)
- `endpoint`: `browser` or `mpc`
- `follow_url`: browser follower auto navigation
- sync parameters (`tick_ms`, `deadzone_ms`, etc)
- MPC-BE Web UI config (`mpc.*`)

To test multiple local clients, set each client's `ext_listen_addr` to a different
port and adjust the extension popup `Client Port` to match.

## Regenerate protobufs

If you change `proto/videowithyou.proto`:

```
protoc --proto_path=proto --plugin=protoc-gen-go=$env:GOPATH\bin\protoc-gen-go.exe --go_out=proto/gen --go_opt=paths=source_relative videowithyou.proto
```

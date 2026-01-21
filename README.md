# VideoWithYou v2

Multi-part system for synchronized video playback. The extension only talks to the local client (local WebSocket); the local client talks to the server (WebSocket with protobuf).

## Structure

- `proto/` Protobuf definitions and generated Go code
- `server/` Go WebSocket server
- `local-client/` Go local client + extension bridge
- `extension/` MV3 extension (TypeScript + Vite)
- `scripts/` build + install helpers

## Build (Windows)

From `v2/`:

```
.\scripts\build.ps1
```

This builds `bin/server.exe`, `bin/local-client.exe`, and `bin/server-linux`.

## Run

1) Start server:

```
./bin/server.exe
```

2) Build the extension:

```
cd extension
npm install
npm run build
```

3) Load extension:
- Edge/Chrome -> Extensions -> Load unpacked -> `v2/extension/dist`

4) Start local client manually:

```
./bin/local-client.exe
```

Open the popup in the browser to create/join rooms.

## Local Client Config

Edit `v2/local-client/config.json`:

- `endpoint`: `browser` or `mpc`
- `follow_url`: only applies for `browser`
- `ext_listen_addr` / `ext_listen_path`: extension bridge endpoint
- `ext_idle_timeout_sec`: browser endpoint idle window (0 disables)
- `endpoint_inactive_timeout_sec`: follower leave timeout after endpoint missing (0 disables)
- Sync knobs: `tick_ms`, `deadzone_ms`, `hard_seek_threshold_ms`, `soft_rate_*`, `offset_ms`
- MPC-BE (Web UI):
  - Enable Web UI in MPC-BE settings (Web Interface) and set a port.
  - `mpc.base_url`: MPC-BE Web UI base URL (e.g. `http://127.0.0.1:13579`)
  - `mpc.variables_path`: defaults to `/variables.html`
  - `mpc.commands.*`: command templates (relative or absolute). Use `POST /path|body` for form posts. Placeholders: `{ms}`, `{sec}`, `{hhmmss}`, `{hhmmssms}`, `{rate}`.
  - MPC mode does not sync playback rate (pause/seek only).

The client will persist config updates triggered from the UI.

## Multi-Client Local Test

Run each local client on a different port via `ext_listen_addr` (e.g. `127.0.0.1:23333` and `127.0.0.1:23334`), then set the extension popup `Client Port` to match in each browser (Edge/Chrome).

## Notes

- `apply_state.position_ms` uses `-1` to signal "no seek" for rate-only adjustments.
- Time sync uses NTP-style 4 timestamps; initial 5 samples pick the lowest-delay offset.
- MPC-BE integration uses its Web UI. If commands do not work, adjust `mpc.commands` based on your MPC-BE Web UI.
- Server closes rooms if the host stops reporting for `-host_idle_timeout_sec` (default 600s).

## Protobuf

If you change the schema:

```
protoc --proto_path=proto --plugin=protoc-gen-go=$env:GOPATH\bin\protoc-gen-go.exe --go_out=proto/gen --go_opt=paths=source_relative videowithyou.proto
```

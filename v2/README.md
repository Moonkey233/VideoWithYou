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

- `endpoint`: `browser` or `potplayer`
- `follow_url`: only applies for `browser`
- `ext_listen_addr` / `ext_listen_path`: extension bridge endpoint
- `ext_idle_timeout_sec`: auto leave room if no extension traffic (0 disables)
- Sync knobs: `tick_ms`, `deadzone_ms`, `hard_seek_threshold_ms`, `soft_rate_*`, `offset_ms`
- PotPlayer:
  - `potplayer.path`: full path to `PotPlayerMini64.exe`
  - `potplayer.hotkeys`: hotkey strings (e.g. `CTRL+UP`)

The client will persist config updates triggered from the UI.

## Multi-Client Local Test

Run each local client on a different port via `ext_listen_addr` (e.g. `127.0.0.1:27111` and `127.0.0.1:27112`), then set the extension popup `Client Port` to match in each browser (Edge/Chrome).

## Notes

- `apply_state.position_ms` uses `-1` to signal "no seek" for rate-only adjustments.
- Time sync uses NTP-style 4 timestamps; initial 5 samples pick the lowest-delay offset.
- PotPlayer integration is best-effort (seek via command line + hotkeys for play/pause/rate).

## Protobuf

If you change the schema:

```
protoc --plugin=protoc-gen-go=$env:GOPATH\bin\protoc-gen-go.exe --go_out=proto/gen --go_opt=paths=source_relative proto/videowithyou.proto
```

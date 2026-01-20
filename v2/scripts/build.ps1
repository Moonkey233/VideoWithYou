$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $root
try {
  go build -o bin/server.exe ./server/cmd/server
  go build -o bin/local-client.exe ./local-client/cmd/local-client

  $prevGoos = $env:GOOS
  $prevGoarch = $env:GOARCH
  $env:GOOS = "linux"
  $env:GOARCH = "amd64"
  go build -o bin/server-linux ./server/cmd/server
  $env:GOOS = $prevGoos
  $env:GOARCH = $prevGoarch

  Push-Location extension
  try {
    npm run build
  } finally {
    Pop-Location
  }
} finally {
  Pop-Location
}

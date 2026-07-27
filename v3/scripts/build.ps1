$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$binDir = Join-Path $root "bin"
$releaseDir = Join-Path $root "release"
$extensionDir = Join-Path $root "extension"
$extensionDist = Join-Path $extensionDir "dist"
$embeddedExtension = Join-Path $root "local-client\internal\embedded\extension"

Push-Location $root
try {
  New-Item -ItemType Directory -Force -Path $binDir, $releaseDir | Out-Null

  Push-Location $extensionDir
  try {
    if (-not (Test-Path (Join-Path $extensionDir "node_modules"))) {
      npm ci
    }
    npm run typecheck
    npm run build
  } finally {
    Pop-Location
  }

  $resolvedEmbeddedParent = (Resolve-Path (Split-Path $embeddedExtension -Parent)).Path
  if (-not $resolvedEmbeddedParent.StartsWith($root, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Unsafe embedded extension path: $embeddedExtension"
  }
  if (Test-Path -LiteralPath $embeddedExtension) {
    Remove-Item -LiteralPath $embeddedExtension -Recurse -Force
  }
  New-Item -ItemType Directory -Path $embeddedExtension | Out-Null
  Copy-Item -Path (Join-Path $extensionDist "*") -Destination $embeddedExtension -Recurse -Force

  $iconScript = Join-Path $root "scripts\make_icons.py"
  $iconSource = Join-Path $root "VideoWithYou.png"
  if ((Test-Path $iconScript) -and (Test-Path $iconSource)) {
    $python = Get-Command python -ErrorAction SilentlyContinue
    if (-not $python) {
      $python = Get-Command py -ErrorAction SilentlyContinue
    }
    if ($python) {
      if ($python.Name -eq "py") {
        & $python.Source -3 $iconScript --src $iconSource
      } else {
        & $python.Source $iconScript --src $iconSource
      }
    }
  }

  $rsrc = Get-Command rsrc -ErrorAction SilentlyContinue
  $icoPath = Join-Path $root "local-client\assets\client.ico"
  $rsrcOut = Join-Path $root "local-client\cmd\local-client\rsrc.syso"
  if ($rsrc -and (Test-Path $icoPath)) {
    & $rsrc.Source -arch amd64 -ico $icoPath -o $rsrcOut
  }

  go test ./...
  go vet ./...
  go build -trimpath -ldflags "-s -w" -o (Join-Path $binDir "VideoWithYou.exe") ./local-client/cmd/local-client
  go build -trimpath -ldflags "-s -w" -o (Join-Path $binDir "server-standalone.exe") ./server/cmd/server

  $previousGoos = $env:GOOS
  $previousGoarch = $env:GOARCH
  try {
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    go build -trimpath -ldflags "-s -w" -o (Join-Path $binDir "server-standalone-linux-amd64") ./server/cmd/server
  } finally {
    $env:GOOS = $previousGoos
    $env:GOARCH = $previousGoarch
  }

  $releaseExe = Join-Path $releaseDir "VideoWithYou-v3-windows-amd64.exe"
  Copy-Item -LiteralPath (Join-Path $binDir "VideoWithYou.exe") -Destination $releaseExe -Force
  $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $releaseExe).Hash.ToLowerInvariant()
  Set-Content -LiteralPath (Join-Path $releaseDir "VideoWithYou-v3-windows-amd64.sha256") -Value "$hash  VideoWithYou-v3-windows-amd64.exe" -Encoding ascii

  Write-Host "Build complete"
  Write-Host "Windows release: $releaseExe"
  Write-Host "SHA256: $hash"
} finally {
  Pop-Location
}

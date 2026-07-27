$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $root
try {
  $iconScript = Join-Path $root "scripts/make_icons.py"
  $iconSource = Join-Path $root "VideoWithYou.png"
  if ((Test-Path $iconScript) -and (Test-Path $iconSource)) {
    $py = Get-Command python -ErrorAction SilentlyContinue
    if (-not $py) {
      $py = Get-Command py -ErrorAction SilentlyContinue
    }
    if ($py) {
      if ($py.Name -eq "py") {
        & $py.Source -3 $iconScript --src $iconSource
      } else {
        & $py.Source $iconScript --src $iconSource
      }
    } else {
      Write-Host "python not found, skip icon generation"
    }
  }

  $rsrc = Get-Command rsrc -ErrorAction SilentlyContinue
  $icoPath = Join-Path $root "local-client/assets/client.ico"
  $rsrcOut = Join-Path $root "local-client/cmd/local-client/rsrc.syso"
  if ($rsrc -and (Test-Path $icoPath)) {
    & $rsrc.Source -arch amd64 -ico $icoPath -o $rsrcOut
  } else {
    Write-Host "rsrc not found or icon missing, skip client icon embedding"
  }

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

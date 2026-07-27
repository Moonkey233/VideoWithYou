$ErrorActionPreference = "Continue"

Write-Host "VideoWithYou v3 network verification"
Write-Host ""

$aaaa = Resolve-DnsName ipv6.moonkey.top -Type AAAA -ErrorAction SilentlyContinue
if ($aaaa) {
  Write-Host "[OK] ipv6.moonkey.top AAAA:"
  $aaaa | Where-Object IPAddress | ForEach-Object { Write-Host "     $($_.IPAddress)" }
} else {
  Write-Host "[FAIL] ipv6.moonkey.top has no reachable AAAA response"
}

$cloudA = Resolve-DnsName moonkey.top -Type A -ErrorAction SilentlyContinue
if ($cloudA) {
  Write-Host "[OK] moonkey.top A:"
  $cloudA | Where-Object IPAddress | ForEach-Object { Write-Host "     $($_.IPAddress)" }
} else {
  Write-Host "[FAIL] moonkey.top has no reachable A response"
}

$direct = Test-NetConnection ipv6.moonkey.top -Port 21314 -InformationLevel Detailed
if ($direct.TcpTestSucceeded) {
  Write-Host "[OK] IPv6 direct TCP 21314 is reachable"
} else {
  Write-Host "[WARN] IPv6 direct TCP 21314 is not reachable from this computer"
}

$cloud = Test-NetConnection moonkey.top -Port 21314 -InformationLevel Detailed
if ($cloud.TcpTestSucceeded) {
  Write-Host "[OK] Cloud IPv4 relay TCP 21314 is reachable"
} else {
  Write-Host "[WARN] Cloud IPv4 relay TCP 21314 is not reachable"
}

$ErrorActionPreference = "Stop"
$repo = if ($env:CLIKS_REPO) { $env:CLIKS_REPO } else { "YashMahawa/Cliks" }
$binDir = if ($env:CLIKS_BIN_DIR) { $env:CLIKS_BIN_DIR } else { Join-Path $env:LOCALAPPDATA "Cliks\bin" }
$asset = "cliks-windows-amd64.zip"
$requiredVersion = [version]"0.6.12"
$url = "https://github.com/$repo/releases/latest/download/$asset"
$temp = Join-Path ([IO.Path]::GetTempPath()) ("cliks-" + [Guid]::NewGuid())

Write-Host "Installing Cliks..."
New-Item -ItemType Directory -Force -Path $temp, $binDir | Out-Null
try {
  $archive = Join-Path $temp $asset
  Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $archive
  Expand-Archive -Force -Path $archive -DestinationPath $temp
  $downloaded = Join-Path $temp "cliks.exe"
  $downloadedVersion = [version]((& $downloaded version).Trim())
  if ($downloadedVersion -lt $requiredVersion) {
    throw "Latest published release is $downloadedVersion, but $requiredVersion or newer is required for embedded sounds."
  }
  Copy-Item -Force $downloaded (Join-Path $binDir "cliks.exe")
} catch {
  throw "Could not download the latest Cliks release. Check https://github.com/$repo/releases and try again. $($_.Exception.Message)"
} finally {
  Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $temp
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($userPath -split ";") -notcontains $binDir) {
  $nextPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $binDir } else { "$userPath;$binDir" }
  [Environment]::SetEnvironmentVariable("Path", $nextPath, "User")
}
$env:Path = "$binDir;$env:Path"

Write-Host "[ok] Installed: $binDir\cliks.exe"
Write-Host "[ok] Version $downloadedVersion (bundled sounds included)"
Write-Host "Running the easy setup check..."
& (Join-Path $binDir "cliks.exe") setup
Write-Host ""
Write-Host "Cliks is ready. Join your room with:"
Write-Host "  cliks join CLIK-XXXXXX"
Write-Host "If this window cannot find cliks later, open a new PowerShell window."

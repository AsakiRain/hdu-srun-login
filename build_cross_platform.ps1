param(
    [switch]$SkipTests,
    [switch]$NoClean
)

$ErrorActionPreference = "Stop"
$AppName = "hdu-srun-login"
$BuildDir = "build"

$Targets = @(
    @{ GOOS = "linux";   GOARCH = "amd64";  Output = (Join-Path $BuildDir "$AppName-linux-amd64") },
    @{ GOOS = "linux";   GOARCH = "arm64";  Output = (Join-Path $BuildDir "$AppName-linux-arm64") },
    @{ GOOS = "linux";   GOARCH = "mipsle"; Output = (Join-Path $BuildDir "$AppName-linux-mipsle") },
    @{ GOOS = "darwin";  GOARCH = "amd64";  Output = (Join-Path $BuildDir "$AppName-darwin-amd64") },
    @{ GOOS = "darwin";  GOARCH = "arm64";  Output = (Join-Path $BuildDir "$AppName-darwin-arm64") },
    @{ GOOS = "windows"; GOARCH = "amd64";  Output = (Join-Path $BuildDir "$AppName-windows-amd64.exe") }
)

if (-not $SkipTests) {
    Write-Host "==> Running tests"
    go test ./...
}

if (-not $NoClean) {
    Write-Host "==> Cleaning old release binaries"
    if (Test-Path -LiteralPath $BuildDir) {
        Remove-Item -LiteralPath $BuildDir -Recurse -Force
    }
}
New-Item -ItemType Directory -Path $BuildDir -Force | Out-Null

$OldGOOS = $env:GOOS
$OldGOARCH = $env:GOARCH
$OldCGO = $env:CGO_ENABLED

try {
    foreach ($Target in $Targets) {
        $env:GOOS = $Target.GOOS
        $env:GOARCH = $Target.GOARCH
        $env:CGO_ENABLED = "0"

        Write-Host "==> Building $($Target.GOOS)/$($Target.GOARCH) -> $($Target.Output)"
        go build -trimpath -ldflags="-s -w" -o $Target.Output .
    }
}
finally {
    $env:GOOS = $OldGOOS
    $env:GOARCH = $OldGOARCH
    $env:CGO_ENABLED = $OldCGO
}

Write-Host "==> Done"
Get-Item -LiteralPath ($Targets | ForEach-Object { $_.Output }) |
    Select-Object Name, Length, LastWriteTime

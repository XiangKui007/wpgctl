# wpgctl build script (Windows PowerShell)
# Usage (run from repo root):
#   powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
#   powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 -Target release

param(
    [ValidateSet('build', 'build-linux', 'build-arm64', 'ui-build', 'release', 'test')]
    [string]$Target = 'build-linux'
)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

$App = 'wpgctl'
$Module = 'github.com/wpg/wpgctl'
$BinDir = Join-Path $PWD 'bin'

function Get-VersionInfo {
    $version = 'dev'
    $commit = 'unknown'
    try {
        $v = git describe --tags --always --dirty 2>$null
        if ($v) { $version = $v.Trim() }
    } catch { }
    try {
        $c = git rev-parse --short HEAD 2>$null
        if ($c) { $commit = $c.Trim() }
    } catch { }
    $built = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
    return @{ Version = $version; Commit = $commit; Built = $built }
}

function Get-LdFlags {
    $info = Get-VersionInfo
    return "-s -w -X ${Module}/internal/version.Version=$($info.Version) -X ${Module}/internal/version.GitCommit=$($info.Commit) -X ${Module}/internal/version.BuildTime=$($info.Built)"
}

function Ensure-BinDir {
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
}

function Invoke-UiBuild {
    Write-Host '>> npm run build (web/)' -ForegroundColor Cyan
    Push-Location web
    try {
        if (-not (Get-Command npm.cmd -ErrorAction SilentlyContinue)) {
            throw 'npm.cmd not found; install Node.js first'
        }
        & npm.cmd install
        if ($LASTEXITCODE -ne 0) { throw "npm install failed (exit $LASTEXITCODE)" }
        & npm.cmd run build
        if ($LASTEXITCODE -ne 0) { throw "npm run build failed (exit $LASTEXITCODE)" }
    } finally {
        Pop-Location
    }
}

function Invoke-GoBuild {
    param(
        [string]$GoOs,
        [string]$GoArch,
        [string]$OutName
    )
    Ensure-BinDir
    $out = Join-Path $BinDir $OutName
    $ldflags = Get-LdFlags
    Write-Host ">> go build GOOS=$GoOs GOARCH=$GoArch -> $out" -ForegroundColor Cyan

    $env:GOOS = $GoOs
    $env:GOARCH = $GoArch
    $env:CGO_ENABLED = '0'
    if (-not $env:GOPROXY) {
        $env:GOPROXY = 'https://goproxy.cn,direct'
    }

    & go build -ldflags $ldflags -o $out ./cmd/wpgctl
    if ($LASTEXITCODE -ne 0) { throw "go build failed (exit $LASTEXITCODE)" }

    $item = Get-Item $out
    $mb = [math]::Round($item.Length / 1MB, 2)
    Write-Host "   done: $($item.FullName) ($mb MB)" -ForegroundColor Green
}

switch ($Target) {
    'ui-build' {
        Invoke-UiBuild
    }
    'build' {
        Ensure-BinDir
        Remove-Item Env:GOOS -ErrorAction SilentlyContinue
        Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
        $env:CGO_ENABLED = '0'
        $exe = '.exe'
        try {
            $goexe = (& go env GOEXE 2>$null)
            if ($goexe) { $exe = $goexe }
        } catch { }
        $out = Join-Path $BinDir "${App}${exe}"
        $ldflags = Get-LdFlags
        Write-Host ">> go build (windows) -> $out" -ForegroundColor Cyan
        if (-not $env:GOPROXY) { $env:GOPROXY = 'https://goproxy.cn,direct' }
        & go build -ldflags $ldflags -o $out ./cmd/wpgctl
        if ($LASTEXITCODE -ne 0) { throw "go build failed (exit $LASTEXITCODE)" }
        Write-Host "   done: $out" -ForegroundColor Green
    }
    'build-linux' {
        Invoke-GoBuild -GoOs 'linux' -GoArch 'amd64' -OutName "${App}-linux-amd64"
    }
    'build-arm64' {
        Invoke-GoBuild -GoOs 'linux' -GoArch 'arm64' -OutName "${App}-linux-arm64"
    }
    'release' {
        Invoke-UiBuild
        Invoke-GoBuild -GoOs 'linux' -GoArch 'amd64' -OutName "${App}-linux-amd64"
        Invoke-GoBuild -GoOs 'linux' -GoArch 'arm64' -OutName "${App}-linux-arm64"
        Write-Host ''
        Write-Host 'Artifacts in bin/:' -ForegroundColor Yellow
        Get-ChildItem $BinDir -Filter "${App}-linux-*" | ForEach-Object { Write-Host "  $($_.Name)" }
        Write-Host ''
        Write-Host 'On Linux: chmod +x wpgctl-linux-amd64' -ForegroundColor Yellow
        Write-Host '          ./wpgctl-linux-amd64 ui --site site.yaml' -ForegroundColor Yellow
    }
    'test' {
        Write-Host '>> go test ./...' -ForegroundColor Cyan
        & go test ./...
        if ($LASTEXITCODE -ne 0) { throw "tests failed (exit $LASTEXITCODE)" }
    }
}

Write-Host 'OK' -ForegroundColor Green

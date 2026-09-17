# Windows 侧：构建发布包并提示上传现场（无需 make）
# 用法：
#   .\scripts\deploy.ps1              # 编前端 + Linux amd64，并列出待上传文件
#   .\scripts\deploy.ps1 -SkipUi      # 跳过前端（未改 UI 时）
#   .\scripts\deploy.ps1 -ScpTo user@192.168.1.100:/opt/wpgctl/   # 可选：scp 上传

param(
    [switch]$SkipUi,
    [string]$ScpTo = ''
)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

if (-not $SkipUi) {
    & "$PSScriptRoot\build.ps1" -Target ui-build
} else {
    Write-Host '>> 跳过前端构建 (-SkipUi)' -ForegroundColor Yellow
}

& "$PSScriptRoot\build.ps1" -Target build-linux

$bin = Join-Path $Root 'bin\wpgctl-linux-amd64'
if (-not (Test-Path $bin)) {
    throw "未找到 $bin"
}

Write-Host ''
Write-Host '======== 待上传到 Linux 主控机 ========' -ForegroundColor Yellow
Write-Host "  $bin"
Write-Host "  site.yaml（现场配置，可用 configs/examples/site.example.yaml 改）"
Write-Host ''
Write-Host 'Linux 现场执行：' -ForegroundColor Yellow
Write-Host '  chmod +x wpgctl-linux-amd64'
Write-Host '  ./wpgctl-linux-amd64 ui --site site.yaml'
Write-Host '  # 或 CLI 一条龙：'
Write-Host '  ./scripts/deploy-linux.sh all --site site.yaml --package /path/to/release --base /path/to/base'
Write-Host ''

if ($ScpTo) {
    if (-not (Get-Command scp -ErrorAction SilentlyContinue)) {
        Write-Warning '未找到 scp，请手动上传'
    } else {
        Write-Host ">> scp $bin -> ${ScpTo}" -ForegroundColor Cyan
        & scp $bin "${ScpTo}/wpgctl-linux-amd64"
        Write-Host '   上传完成' -ForegroundColor Green
    }
}

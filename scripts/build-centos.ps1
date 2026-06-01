param(
    [string]$Version = "dev",
    [string]$OutputDir = "build/centos"
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$outputPath = Join-Path $repoRoot $OutputDir
New-Item -ItemType Directory -Force $outputPath | Out-Null

try {
    $commit = (git -C $repoRoot rev-parse --short HEAD 2>$null).Trim()
    if (-not $commit) {
        $commit = "unknown"
    }
} catch {
    $commit = "unknown"
}

$buildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

docker build `
    --target export `
    --platform linux/amd64 `
    --output "type=local,dest=$outputPath" `
    --build-arg "VERSION=$Version" `
    --build-arg "COMMIT=$commit" `
    --build-arg "BUILD_TIME=$buildTime" `
    -f (Join-Path $repoRoot "Dockerfile") `
    $repoRoot

Write-Host "CentOS/Linux artifact generated under: $outputPath"
Get-ChildItem $outputPath

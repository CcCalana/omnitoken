param(
    [ValidateSet("help", "up", "down", "logs", "test", "lint", "format", "docker-build")]
    [string] $Task = "help"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root
$Compose = @("compose", "--env-file", ".env", "-f", "deploy/docker-compose.yml")

function Show-Help {
    Write-Host "OmniToken commands:"
    Write-Host "  .\scripts\dev.ps1 up            Start local services"
    Write-Host "  .\scripts\dev.ps1 down          Stop local services"
    Write-Host "  .\scripts\dev.ps1 logs          Follow compose logs"
    Write-Host "  .\scripts\dev.ps1 test          Run Go tests"
    Write-Host "  .\scripts\dev.ps1 lint          Run Go vet"
    Write-Host "  .\scripts\dev.ps1 format        Format Go code"
    Write-Host "  .\scripts\dev.ps1 docker-build  Build gateway, admin, and migrate images"
}

switch ($Task) {
    "help" {
        Show-Help
    }
    "docker-build" {
        docker build -f deploy/Dockerfile.gateway -t omnitoken-gateway:local .
        docker build -f deploy/Dockerfile.admin -t omnitoken-admin:local .
        docker build -f deploy/Dockerfile.migrate -t omnitoken-migrate:local .
    }
    "up" {
        docker build -f deploy/Dockerfile.gateway -t omnitoken-gateway:local .
        docker build -f deploy/Dockerfile.admin -t omnitoken-admin:local .
        docker build -f deploy/Dockerfile.migrate -t omnitoken-migrate:local .
        docker @Compose up -d --no-build
    }
    "down" {
        docker @Compose down
    }
    "logs" {
        docker @Compose logs -f
    }
    "test" {
        go test ./...
    }
    "lint" {
        go vet ./...
    }
    "format" {
        go fmt ./...
    }
}

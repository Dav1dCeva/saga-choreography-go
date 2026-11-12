#requires -Version 5.1
<#
.SYNOPSIS
    Comandos del proyecto para Windows usando Docker Desktop.
.DESCRIPTION
    Equivalente Windows/PowerShell del Makefile. Usa "docker compose" y no
    requiere make, Bash ni Python.
.EXAMPLE
    .\scripts\saga.ps1 up
    .\scripts\saga.ps1 demo-ok
    .\scripts\saga.ps1 e2e -Scenario shipping-fail
#>
[CmdletBinding()]
param(
    [ValidateSet(
        'help', 'up', 'down', 'ps', 'logs', 'build', 'test', 'vet', 'cover',
        'cover-core', 'integration', 'trace', 'e2e', 'demo-ok', 'demo-payment-fail',
        'demo-inventory-fail', 'demo-shipping-fail', 'reset', 'sonar', 'sonar-down'
    )]
    [string]$Command = 'help',

    [ValidateSet('ok', 'payment-fail', 'inventory-fail', 'shipping-fail')]
    [string]$Scenario = 'ok',

    [string]$OrderId = ''
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$e2e = Join-Path $PSScriptRoot 'e2e.ps1'
$composeFiles = @(
    '-f', (Join-Path $root 'docker-compose.yml'),
    '-f', (Join-Path $root 'docker-compose.windows.yml')
)

function Invoke-Compose {
    param([string[]]$ComposeArguments)
    $allArguments = $composeFiles + $ComposeArguments
    & docker compose @allArguments
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose falló con código $LASTEXITCODE"
    }
}

function Invoke-CoreCoverage {
    $core = @(go list ./services/... | Where-Object { $_ -match '/(domain|application)$' })
    if ($core.Count -eq 0) {
        throw 'No se encontraron paquetes domain/application.'
    }
    go test -covermode=atomic -coverprofile=coverage-core.out @core
    if ($LASTEXITCODE -ne 0) {
        throw "go test falló con código $LASTEXITCODE"
    }
    go tool cover -func=coverage-core.out | Select-Object -Last 1
}

Push-Location $root
try {
    switch ($Command) {
        'help' {
            Write-Host 'Comandos disponibles:'
            Write-Host '  .\scripts\saga.cmd up'
            Write-Host '  .\scripts\saga.cmd down'
            Write-Host '  .\scripts\saga.cmd ps'
            Write-Host '  .\scripts\saga.cmd logs'
            Write-Host '  .\scripts\saga.cmd test'
            Write-Host '  .\scripts\saga.cmd cover'
            Write-Host '  .\scripts\saga.cmd cover-core'
            Write-Host '  .\scripts\saga.cmd integration'
            Write-Host '  .\scripts\saga.cmd trace [order_id]'
            Write-Host '  .\scripts\saga.cmd demo-ok'
            Write-Host '  .\scripts\saga.cmd demo-payment-fail'
            Write-Host '  .\scripts\saga.cmd demo-inventory-fail'
            Write-Host '  .\scripts\saga.cmd demo-shipping-fail'
            Write-Host '  .\scripts\saga.cmd reset'
            Write-Host '  .\scripts\saga.cmd sonar'
            Write-Host '  .\scripts\saga.cmd sonar-down'
            Write-Host ''
            Write-Host 'También se puede usar directamente: docker compose -f docker-compose.yml -f docker-compose.windows.yml up -d --build'
        }
        'up' {
            Invoke-Compose -ComposeArguments @('up', '-d', '--build')
        }
        'down' {
            Invoke-Compose -ComposeArguments @('down', '-v', '--remove-orphans')
        }
        'ps' {
            Invoke-Compose -ComposeArguments @('ps')
        }
        'logs' {
            Invoke-Compose -ComposeArguments @('logs', '-f', '--tail=100')
        }
        'build' {
            go build ./...
            if ($LASTEXITCODE -ne 0) { throw "go build falló con código $LASTEXITCODE" }
        }
        'test' {
            go test ./...
            if ($LASTEXITCODE -ne 0) { throw "go test falló con código $LASTEXITCODE" }
        }
        'vet' {
            go vet ./...
            if ($LASTEXITCODE -ne 0) { throw "go vet falló con código $LASTEXITCODE" }
        }
        'cover' {
            go test -race -covermode=atomic -coverprofile=coverage.out ./...
            if ($LASTEXITCODE -ne 0) { throw "go test falló con código $LASTEXITCODE" }
            go tool cover -func=coverage.out | Select-Object -Last 1
        }
        'cover-core' {
            Invoke-CoreCoverage
        }
        'integration' {
            go test -tags=integration -count=1 ./pkg/broker ./services/order-service/internal/adapters/persistence
            if ($LASTEXITCODE -ne 0) { throw "pruebas de integración fallaron con código $LASTEXITCODE" }
        }
        'trace' {
            & (Join-Path $PSScriptRoot 'trace.ps1') -OrderId $OrderId
            if (-not $?) { exit 1 }
        }
        'e2e' {
            & $e2e -Scenario $Scenario
            if (-not $?) { exit 1 }
        }
        'demo-ok' {
            & $e2e -Scenario ok
            if (-not $?) { exit 1 }
        }
        'demo-payment-fail' {
            & $e2e -Scenario payment-fail
            if (-not $?) { exit 1 }
        }
        'demo-inventory-fail' {
            & $e2e -Scenario inventory-fail
            if (-not $?) { exit 1 }
        }
        'demo-shipping-fail' {
            & $e2e -Scenario shipping-fail
            if (-not $?) { exit 1 }
        }
        'reset' {
            $inventorySql = "UPDATE stock SET available = CASE sku WHEN 'laptop' THEN 3 WHEN 'phone' THEN 10 WHEN 'headphones' THEN 20 WHEN 'monitor' THEN 4 ELSE available END; TRUNCATE reservations;"
            Invoke-Compose -ComposeArguments @('exec', '-T', 'postgres', 'psql', '-U', 'saga', '-d', 'inventory', '-c', $inventorySql)
            Invoke-Compose -ComposeArguments @('exec', '-T', 'postgres', 'psql', '-U', 'saga', '-d', 'orders', '-c', 'TRUNCATE orders;')
            Invoke-Compose -ComposeArguments @('exec', '-T', 'postgres', 'psql', '-U', 'saga', '-d', 'payments', '-c', 'TRUNCATE payments;')
            Invoke-Compose -ComposeArguments @('exec', '-T', 'postgres', 'psql', '-U', 'saga', '-d', 'shipping', '-c', 'TRUNCATE shipments;')
            Write-Host 'Estado de demo restaurado.'
        }
        'sonar' {
            & docker compose -f docker-compose.sonar.yml up -d
            if ($LASTEXITCODE -ne 0) { throw "docker compose (sonar) falló con código $LASTEXITCODE" }
        }
        'sonar-down' {
            & docker compose -f docker-compose.sonar.yml down -v
            if ($LASTEXITCODE -ne 0) { throw "docker compose (sonar) falló con código $LASTEXITCODE" }
        }
    }
}
finally {
    Pop-Location
}

#requires -Version 5.1
<#
.SYNOPSIS
    Ejecuta uno de los cuatro escenarios end-to-end de la saga.
.DESCRIPTION
    Equivalente Windows/PowerShell de scripts/e2e.sh. No requiere Bash,
    curl ni Python; usa Invoke-RestMethod y ConvertFrom-Json.
.EXAMPLE
    .\scripts\e2e.ps1 ok
    .\scripts\e2e.ps1 shipping-fail
#>
[CmdletBinding()]
param(
    [ValidateSet('ok', 'payment-fail', 'inventory-fail', 'shipping-fail')]
    [string]$Scenario = 'ok',

    [string]$OrderUrl = 'http://localhost:8081',

    [int]$TimeoutSec = 60
)

$ErrorActionPreference = 'Stop'

switch ($Scenario) {
    'ok' {
        $payload = @{
            customer  = 'Ana Torres'
            product   = 'phone'
            quantity  = 1
            unitPrice = 400
            address   = 'Av. Siempre Viva 742'
        }
        $expected = 'COMPLETED'
    }
    'payment-fail' {
        $payload = @{
            customer  = 'Ana Torres'
            product   = 'phone'
            quantity  = 1
            unitPrice = 1500
            address   = 'Av. Siempre Viva 742'
        }
        $expected = 'CANCELLED'
    }
    'inventory-fail' {
        $payload = @{
            customer  = 'Ana Torres'
            product   = 'laptop'
            quantity  = 99
            unitPrice = 1
            address   = 'Av. Siempre Viva 742'
        }
        $expected = 'CANCELLED'
    }
    'shipping-fail' {
        $payload = @{
            customer  = 'Ana Torres'
            product   = 'phone'
            quantity  = 1
            unitPrice = 400
            address   = 'direccion invalid'
        }
        $expected = 'CANCELLED'
    }
}

$body = $payload | ConvertTo-Json -Compress
try {
    $order = Invoke-RestMethod -Method Post -Uri "$OrderUrl/orders" `
        -ContentType 'application/json; charset=utf-8' -Body $body
}
catch {
    Write-Error "No se pudo crear el pedido. ¿Está levantado order-service en $OrderUrl? Detalle: $($_.Exception.Message)"
    exit 1
}

$orderId = $order.id
if ([string]::IsNullOrWhiteSpace($orderId)) {
    Write-Error "Respuesta inesperada al crear el pedido: $($order | ConvertTo-Json -Depth 8)"
    exit 1
}

Write-Host "Escenario: $Scenario"
Write-Host "Pedido creado: $orderId"
Write-Host "Estado esperado: $expected"

$lastStatus = ''
$deadline = (Get-Date).AddSeconds($TimeoutSec)
while ((Get-Date) -lt $deadline) {
    try {
        $current = Invoke-RestMethod -Method Get -Uri "$OrderUrl/orders/$orderId"
        $status = [string]$current.status
        if ($status -ne $lastStatus) {
            Write-Host "  estado: $status"
            $lastStatus = $status
        }
        if ($status -eq $expected) {
            Write-Host "OK: $orderId terminó en $status"
            $current | ConvertTo-Json -Depth 8
            exit 0
        }
    }
    catch {
        # El pedido puede tardar en aparecer o la API puede estar reiniciando.
    }
    Start-Sleep -Seconds 2
}

Write-Error "TIMEOUT: el pedido $orderId quedó en '$lastStatus' y se esperaba '$expected'"
exit 1

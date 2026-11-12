#requires -Version 5.1
<#
.SYNOPSIS
    Muestra la traza legible de una saga en Windows con Docker Desktop.
.DESCRIPTION
    Equivalente a scripts/trace.sh. Si no se indica OrderId, toma el último
    pedido creado. Usa docker-compose.yml + docker-compose.windows.yml.
.EXAMPLE
    .\scripts\trace.ps1
    .\scripts\trace.ps1 9f06c97d-ec4f-4e1b-b128-faa8b7288562
#>
[CmdletBinding()]
param(
    [string]$OrderId = ''
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$composeArgs = @(
    '-f', (Join-Path $root 'docker-compose.yml'),
    '-f', (Join-Path $root 'docker-compose.windows.yml'),
    'logs', '--no-color',
    'order-service', 'payment-service', 'inventory-service', 'shipping-service'
)

Push-Location $root
try {
    $raw = & docker compose @composeArgs 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose logs falló con código $LASTEXITCODE"
    }
    $lines = @($raw | ForEach-Object { [string]$_ })

    if ([string]::IsNullOrWhiteSpace($OrderId)) {
        $candidate = $lines |
            Where-Object { $_ -match '"type":"order.created"' -and $_ -match '"msg":"evento publicado"' } |
            Select-Object -Last 1
        if ($candidate) {
            $match = [regex]::Match($candidate, '"order_id":"([^"]+)"')
            if ($match.Success) {
                $OrderId = $match.Groups[1].Value
            }
        }
    }
    if ([string]::IsNullOrWhiteSpace($OrderId)) {
        throw 'No se encontró ningún pedido en los logs.'
    }

    $rows = @()
    foreach ($line in $lines) {
        if ($line -notmatch [regex]::Escape($OrderId)) {
            continue
        }
        $match = [regex]::Match($line, '\{.*\}')
        if (-not $match.Success) {
            continue
        }
        try {
            $rows += ($match.Value | ConvertFrom-Json)
        }
        catch {
            continue
        }
    }
    if ($rows.Count -eq 0) {
        throw 'Sin eventos para ese pedido.'
    }

    $rows = @($rows | Sort-Object { [string]$_.time })
    Write-Host ('SAGA ' + [string]$rows[0].saga_id)
    Write-Host ('{0,-13}{1,-11}{2,-4}{3}' -f 'HORA', 'SERVICIO', '', 'DETALLE')
    Write-Host ('-' * 78)

    foreach ($d in $rows) {
        $time = [string]$d.time
        if ($time.Length -ge 23) {
            $hora = $time.Substring(11, 12)
        }
        else {
            $hora = $time
        }
        $srv = ([string]$d.service).Replace('-service', '')
        $msg = [string]$d.msg
        $icono = '  '
        $txt = $msg

        switch ($msg) {
            'evento recibido' {
                $icono = '<-'
                $txt = 'recibe    ' + [string]$d.type
                break
            }
            'evento publicado' {
                $icono = '->'
                $txt = 'publica   ' + [string]$d.type
                break
            }
            'acción ejecutada' {
                $icono = '**'
                $txt = 'ACCIÓN    ' + [string]$d.action
                if ($d.estado) {
                    $txt += '  => estado ' + [string]$d.estado
                }
                elseif ($d.detalle -or $d.product) {
                    if ($d.detalle) {
                        $detalle = [string]$d.detalle
                    }
                    else {
                        $detalle = [string]$d.product
                    }
                    $txt += '  (' + $detalle + ')'
                }
                if ($d.motivo) {
                    $txt += '  [motivo: ' + [string]$d.motivo + ']'
                }
                break
            }
            'saga completada' {
                $icono = 'OK'
                $txt = 'SAGA COMPLETADA  tracking ' + [string]$d.tracking
                break
            }
            default {
                if ($d.estado -eq 'CANCELLED') {
                    $icono = 'XX'
                    $txt = 'SAGA CANCELADA'
                }
            }
        }

        Write-Host ('{0,-13}{1,-11}{2,-4}{3}' -f $hora, $srv, $icono, $txt)
    }
}
finally {
    Pop-Location
}

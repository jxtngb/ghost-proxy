# Actual-config Ghost Proxy E2E test.
# Runs the real cmd/server and cmd/client applications with repository config.
# Verifies matching and mismatched PSKs through SOCKS5.
# Prerequisites: Go 1.27+, Python 3, configs/server.crt, configs/server.key,
# and a trusted localhost certificate for normal TLS verification.

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
Set-Location $repo
$psk = "00112233445566778899aabbccddeeff"
$wrongPsk = "ffeeddccbbaa99887766554433221100"
$serverExe = Join-Path $env:TEMP "ghost-proxy-e2e-server.exe"
$clientExe = Join-Path $env:TEMP "ghost-proxy-e2e-client.exe"
$targetScript = Join-Path $env:TEMP "ghost-proxy-e2e-target.py"
$serverOut = Join-Path $env:TEMP "ghost-proxy-e2e-server.out.log"
$serverErr = Join-Path $env:TEMP "ghost-proxy-e2e-server.err.log"
$clientOut = Join-Path $env:TEMP "ghost-proxy-e2e-client.out.log"
$clientErr = Join-Path $env:TEMP "ghost-proxy-e2e-client.err.log"
$targetOut = Join-Path $env:TEMP "ghost-proxy-e2e-target.out.log"
$targetErr = Join-Path $env:TEMP "ghost-proxy-e2e-target.err.log"
$serverProcess = $null
$clientProcess = $null
$targetProcess = $null

function Wait-TcpPort([string]$HostName, [int]$Port, [int]$TimeoutSeconds = 10) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $client = New-Object System.Net.Sockets.TcpClient
            $task = $client.ConnectAsync($HostName, $Port)
            if ($task.Wait(250) -and $client.Connected) {
                $client.Close()
                return
            }
            $client.Close()
        } catch {}
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)
    throw "Timed out waiting for port $Port"
}

function Wait-TcpPortClosed([string]$HostName, [int]$Port, [int]$TimeoutSeconds = 10) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $connected = $false
        try {
            $client = New-Object System.Net.Sockets.TcpClient
            $task = $client.ConnectAsync($HostName, $Port)
            $connected = $task.Wait(250) -and $client.Connected
            $client.Close()
        } catch {
            $connected = $false
        }

        if (-not $connected) {
            return
        }

        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)

    throw "Port $Port is still accepting connections"
}

function Stop-TestClientAndWait([System.Diagnostics.Process]$Process, [string]$ExecutablePath) {
    if ($Process -and -not $Process.HasExited) {
        Stop-Process -Id $Process.Id -Force -ErrorAction SilentlyContinue
        try { $Process.WaitForExit(5000) } catch {}
    }

    # The E2E binary has a unique name in TEMP. Kill any leftover instance
    # explicitly because Win32_Process.ExecutablePath can be unavailable.
    $processName = [System.IO.Path]::GetFileNameWithoutExtension($ExecutablePath)
    Get-Process -Name $processName -ErrorAction SilentlyContinue |
        ForEach-Object {
            Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
        }

    $deadline = (Get-Date).AddSeconds(5)
    do {
        $listeners = Get-NetTCPConnection -LocalPort 1080 -State Listen -ErrorAction SilentlyContinue
        if (-not $listeners) { return }

        foreach ($listener in $listeners) {
            $owner = Get-Process -Id $listener.OwningProcess -ErrorAction SilentlyContinue
            if ($owner -and $owner.ProcessName -eq $processName) {
                Stop-Process -Id $owner.Id -Force -ErrorAction SilentlyContinue
            }
        }

        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)
}

function Read-Exact([System.Net.Sockets.NetworkStream]$Stream, [int]$Length) {
    $buffer = New-Object byte[] $Length
    $offset = 0
    while ($offset -lt $Length) {
        $read = $Stream.Read($buffer, $offset, $Length - $offset)
        if ($read -le 0) { throw "Connection closed while reading $Length bytes" }
        $offset += $read
    }
    return $buffer
}

function Invoke-Socks5Request([string]$TargetHost, [int]$TargetPort, [string]$RequestText, [bool]$ExpectSuccess) {
    $tcp = New-Object System.Net.Sockets.TcpClient
    try {
        $tcp.Connect("127.0.0.1", 1080)
        $stream = $tcp.GetStream()
        $stream.ReadTimeout = 5000
        $stream.WriteTimeout = 5000
        $stream.Write([byte[]](0x05, 0x01, 0x00), 0, 3)
        $method = Read-Exact $stream 2
        if ($method[0] -ne 0x05 -or $method[1] -ne 0x00) { throw "Unexpected SOCKS5 method response" }
        $ip = [System.Net.IPAddress]::Parse($TargetHost).GetAddressBytes()
        $request = [byte[]](0x05, 0x01, 0x00, 0x01, $ip[0], $ip[1], $ip[2], $ip[3], [byte](($TargetPort -shr 8) -band 0xff), [byte]($TargetPort -band 0xff))
        $stream.Write($request, 0, $request.Length)
        $reply = Read-Exact $stream 10
        if ($reply[0] -ne 0x05) { throw "Unexpected SOCKS5 version in CONNECT reply" }
        if ($ExpectSuccess) {
            if ($reply[1] -ne 0x00) { throw ("Expected CONNECT success, got 0x{0:X2}" -f $reply[1]) }
            $requestBytes = [System.Text.Encoding]::ASCII.GetBytes($RequestText)
            $stream.Write($requestBytes, 0, $requestBytes.Length)
            $buffer = New-Object byte[] 4096
            $n = $stream.Read($buffer, 0, $buffer.Length)
            if ($n -le 0) { throw "Destination returned no data" }
            return [System.Text.Encoding]::ASCII.GetString($buffer, 0, $n)
        }
        if ($reply[1] -eq 0x00) { throw "Wrong PSK unexpectedly succeeded" }
        Write-Host ("Wrong-PSK CONNECT rejected with SOCKS5 reply 0x{0:X2}" -f $reply[1])
        return $null
    } finally {
        $tcp.Close()
    }
}

try {
    if (-not (Test-Path ".\configs\server.crt")) { throw "configs/server.crt is missing" }
    if (-not (Test-Path ".\configs\server.key")) { throw "configs/server.key is missing" }

    Write-Host "Building actual server and client binaries..."
    go build -o $serverExe .\cmd\server
    if ($LASTEXITCODE -ne 0) { throw "server build failed" }
    go build -o $clientExe .\cmd\client
    if ($LASTEXITCODE -ne 0) { throw "client build failed" }

    $targetCode = @'
import socket
listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
listener.bind(("127.0.0.1", 18080))
listener.listen(5)
while True:
    conn, _ = listener.accept()
    try:
        data = conn.recv(4096)
        if data:
            conn.sendall(b"HTTP/1.1 200 OK\r\n" + b"Content-Length: 12\r\n" + b"Connection: close\r\n\r\n" + b"GHOST-E2E-OK")
    finally:
        conn.close()
'@

    Set-Content -Path $targetScript -Value $targetCode -Encoding UTF8
    Remove-Item $serverOut, $serverErr, $clientOut, $clientErr, $targetOut, $targetErr -Force -ErrorAction SilentlyContinue

    Write-Host "Starting controlled destination on 127.0.0.1:18080..."
    $targetProcess = Start-Process -FilePath "python" -ArgumentList @($targetScript) -RedirectStandardOutput $targetOut -RedirectStandardError $targetErr -PassThru
    Wait-TcpPort "127.0.0.1" 18080

    Write-Host "Starting actual Ghost server using configs/server.yaml..."
    $env:GHOST_PSK = $psk
    $serverProcess = Start-Process -FilePath $serverExe -WorkingDirectory $repo -RedirectStandardOutput $serverOut -RedirectStandardError $serverErr -PassThru
    Wait-TcpPort "127.0.0.1" 443

    Write-Host "Starting actual Ghost client..."
    $env:GHOST_PSK = $psk
    $clientProcess = Start-Process -FilePath $clientExe -WorkingDirectory $repo -RedirectStandardOutput $clientOut -RedirectStandardError $clientErr -PassThru
    Wait-TcpPort "127.0.0.1" 1080

    Write-Host "Testing matching PSK..."
    $httpRequest = "GET / HTTP/1.1" + [char]13 + [char]10 + "Host: localhost" + [char]13 + [char]10 + "Connection: close" + [char]13 + [char]10 + [char]13 + [char]10
    $response = Invoke-Socks5Request "127.0.0.1" 18080 $httpRequest $true
    if ($response -notmatch "GHOST-E2E-OK") { throw "Matching-PSK response did not contain GHOST-E2E-OK" }
    Write-Host "PASS: matching PSK completed the full application path."

    Write-Host "Stopping client before wrong-PSK test..."
    Stop-TestClientAndWait $clientProcess $clientExe
    $clientProcess = $null

    # Ensure no stale test client is still listening on 1080.
    Wait-TcpPortClosed "127.0.0.1" 1080

    Write-Host "Starting client with incorrect PSK..."
    $env:GHOST_PSK = $wrongPsk
    $clientProcess = Start-Process -FilePath $clientExe -WorkingDirectory $repo -RedirectStandardOutput $clientOut -RedirectStandardError $clientErr -PassThru
    Wait-TcpPort "127.0.0.1" 1080

    Write-Host "Testing incorrect PSK..."
    $replyCode = Invoke-Socks5Request "127.0.0.1" 18080 "" $false
    if ($replyCode -eq 0) {
        throw "Wrong PSK unexpectedly succeeded"
    }
    Write-Host ("PASS: incorrect PSK was rejected with SOCKS5 reply 0x{0:X2}." -f $replyCode)
    Write-Host ""
    Write-Host "ACTUAL-CONFIG E2E TEST: PASS"
}
finally {
    $env:GHOST_PSK = $null
    if ($clientProcess -and -not $clientProcess.HasExited) { Stop-Process -Id $clientProcess.Id -Force -ErrorAction SilentlyContinue }
    if ($serverProcess -and -not $serverProcess.HasExited) { Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue }
    if ($targetProcess -and -not $targetProcess.HasExited) { Stop-Process -Id $targetProcess.Id -Force -ErrorAction SilentlyContinue }
}

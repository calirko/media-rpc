$ErrorActionPreference = 'Stop'

Set-Location "$PSScriptRoot\.."

$env:CGO_ENABLED = '1'
go build -ldflags="-s -w -H windowsgui" -o media-rpc.exe .
Write-Host "Built: media-rpc.exe"

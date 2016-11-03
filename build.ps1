# Build script for PowerShell
$ErrorActionPreference = "Stop"

$pwd = (Resolve-Path .\).Path
Write-Host "pwd: " $pwd

docker run --rm -v ${pwd}:/go/src/github.com/iron-io/functions -w /go/src/github.com/iron-io/functions iron/go:dev go build -o functions-alpine
docker build -t iron/functions:latest .

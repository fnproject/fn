# Build script for PowerShell
$ErrorActionPreference = "Stop"

$cmd = $args[0]
Write-Host "cmd: $cmd"

function build () {
    docker run --rm -v ${pwd}:/go/src/github.com/iron-io/functions -w /go/src/github.com/iron-io/functions iron/go:dev go build -o functions-alpine
    docker build -t iron/functions:latest .
}

function run () {
    docker run --rm --privileged -it -e LOG_LEVEL=debug -e "DB=bolt:///app/data/bolt.db" -v ${pwd}/data:/app/data -p 8080:8080 iron/functions
}

switch ($cmd)
{
    "build" { build }
    "run" {run}
    default {"Invalid command: $cmd"}
}

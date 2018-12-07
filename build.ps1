# Build script for PowerShell
$ErrorActionPreference = "Stop"

$cmd = $args[0]
Write-Host "cmd: $cmd"


function quick() {
     try {
        go build
        if (-not $?) {
            Write-Host "build failed!" # WTH, error handling in powershell sucks
            exit
        }
    } catch {
        # This try/catch thing doesn't work, the above if statement does work though
        Write-Host "build failed 2!"
        exit
    }
    ./functions
}

function build () {
    docker run --rm -v ${pwd}:/go/src/github.com/fnproject/fn -w /go/src/github.com/fnproject/fn/cmd/fnserver golang:alpine go build -o fn-alpine
    docker build -t fnproject/fnserver:latest .
}

function run () {
    docker run --rm --name functions -it -v /var/run/docker.sock:/var/run/docker.sock -e FN_LOG_LEVEL=debug -e "FN_DB_URL=sqlite3:///app/data/fn.db" -v $PWD/data:/app/data -p 8080:8080 fnproject/fnserver
}

switch ($cmd)
{
    "quick" {quick}
    "build" { build }
    "run" {run}
    default {"Invalid command: $cmd"}
}

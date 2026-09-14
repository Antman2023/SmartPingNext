$ErrorActionPreference = 'Stop'

if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw 'This helper is for Windows. On other supported platforms, run go test -race ./src/... with a C compiler.'
}
Get-Command go -ErrorAction Stop | Out-Null
Get-Command zig -ErrorAction Stop | Out-Null

$projectRoot = Split-Path -Parent $PSScriptRoot
$environmentNames = @('CGO_ENABLED', 'CC', 'CGO_LDFLAGS', 'GOCACHE', 'ZIG_GLOBAL_CACHE_DIR', 'ZIG_LOCAL_CACHE_DIR')
$savedEnvironment = @{}
foreach ($name in $environmentNames) {
    $savedEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}

$testExitCode = 0
Push-Location -LiteralPath $projectRoot
try {
    $env:CGO_ENABLED = '1'
    $env:CC = 'zig cc'
    $env:CGO_LDFLAGS = ($savedEnvironment['CGO_LDFLAGS'] + ' -lapi-ms-win-core-synch-l1-2-0').Trim()
    if (-not $env:GOCACHE) {
        $env:GOCACHE = Join-Path $projectRoot '.go-cache/go-build'
    }
    if (-not $env:ZIG_GLOBAL_CACHE_DIR) {
        $env:ZIG_GLOBAL_CACHE_DIR = Join-Path $projectRoot '.go-cache/zig-global'
    }
    if (-not $env:ZIG_LOCAL_CACHE_DIR) {
        $env:ZIG_LOCAL_CACHE_DIR = Join-Path $projectRoot '.go-cache/zig-local'
    }
    # Only race test executables use this loading mode. Release builds are unaffected.
    & go test -race '-ldflags=-extldflags=-Wl,--no-dynamicbase' ./src/...
    $testExitCode = $LASTEXITCODE
} finally {
    Pop-Location
    foreach ($name in $environmentNames) {
        if ($null -eq $savedEnvironment[$name]) {
            Remove-Item -LiteralPath "Env:$name" -ErrorAction SilentlyContinue
        } else {
            [Environment]::SetEnvironmentVariable($name, $savedEnvironment[$name], 'Process')
        }
    }
}
if ($testExitCode -ne 0) {
    throw "Race tests failed with exit code $testExitCode."
}

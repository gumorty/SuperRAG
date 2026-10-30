param(
    [ValidateSet('start', 'stop', 'restart', 'status', 'logs')]
    [string]$Action = 'status'
)

$ErrorActionPreference = 'Stop'
$dockerCommand = Get-Command docker -ErrorAction SilentlyContinue
$dockerExecutable = if ($dockerCommand) {
    $dockerCommand.Source
} else {
    Join-Path $env:ProgramFiles 'Docker\Docker\resources\bin\docker.exe'
}

if (-not (Test-Path -LiteralPath $dockerExecutable)) {
    throw 'Docker CLI was not found. Install and start Docker Desktop first.'
}
if (-not (Test-Path -LiteralPath (Join-Path $PSScriptRoot '.env'))) {
    throw 'The deployment .env file is missing. Restore it before starting.'
}

Push-Location $PSScriptRoot
try {
    switch ($Action) {
        'start' { & $dockerExecutable compose up -d --no-build --wait --wait-timeout 240 }
        'stop' { & $dockerExecutable compose stop }
        'restart' {
            & $dockerExecutable compose restart
            if ($LASTEXITCODE -eq 0) {
                & $dockerExecutable compose up -d --no-build --wait --wait-timeout 240
            }
        }
        'status' { & $dockerExecutable compose ps }
        'logs' { & $dockerExecutable compose logs --tail 100 }
    }
    $deploymentExitCode = $LASTEXITCODE
} finally {
    Pop-Location
}
exit $deploymentExitCode

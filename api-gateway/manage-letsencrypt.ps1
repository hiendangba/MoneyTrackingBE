param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("issue", "renew", "local")]
    [string]$Action,

    [string]$Domain,

    [string]$Email,

    [switch]$RestartGateway,

    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$backendDir = Split-Path -Parent $scriptDir
$composeFile = Join-Path $backendDir "compose.yaml"
$certbotConfigDir = Join-Path $scriptDir "letsencrypt"
$certbotWorkDir = Join-Path $scriptDir "lib-letsencrypt"
$envoyCertDir = Join-Path $scriptDir "certs"

function Require-Command {
    param([string]$Name)

    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        if ($Name -eq "mkcert") {
            throw "Missing required command: mkcert. Install it with 'winget install -e --id FiloSottile.mkcert' or 'choco install mkcert -y'."
        }
        throw "Missing required command: $Name"
    }
}

function Generate-LocalCertificates {
    Require-Command "docker"

    $opensslConfigPath = Join-Path $envoyCertDir "openssl-local.cnf"
    $opensslConfig = @"
[req]
default_bits = 2048
prompt = no
default_md = sha256
x509_extensions = v3_req
distinguished_name = dn

[dn]
CN = localhost

[v3_req]
subjectAltName = @alt_names
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth

[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
IP.2 = ::1
"@

    Set-Content -LiteralPath $opensslConfigPath -Value $opensslConfig -NoNewline

    $volumeCerts = "${envoyCertDir}:/work"
    $dockerArgs = @(
        "run", "--rm",
        "-v", $volumeCerts,
        "alpine:3.20",
        "sh", "-lc",
        "apk add --no-cache openssl >/dev/null && openssl req -x509 -nodes -newkey rsa:2048 -days 825 -keyout /work/privkey.pem -out /work/fullchain.pem -config /work/openssl-local.cnf"
    )

    Write-Host "Generating self-signed local certificates with OpenSSL in Docker..."
    & docker @dockerArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Dockerized OpenSSL certificate generation failed with exit code $LASTEXITCODE"
    }

    Remove-Item -LiteralPath $opensslConfigPath -ErrorAction SilentlyContinue
    Write-Host "Copied self-signed local certificates into $envoyCertDir"
}

function Ensure-Directory {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        New-Item -ItemType Directory -Path $Path | Out-Null
    }
}

function Run-DockerCertbot {
    param([string[]]$CertbotArgs)

    $volumeConfig = "${certbotConfigDir}:/etc/letsencrypt"
    $volumeWork = "${certbotWorkDir}:/var/lib/letsencrypt"

    $dockerArgs = @(
        "run", "--rm", "-it",
        "-p", "80:80",
        "-v", $volumeConfig,
        "-v", $volumeWork,
        "certbot/certbot"
    ) + $CertbotArgs

    & docker @dockerArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Certbot failed with exit code $LASTEXITCODE"
    }
}

function Stop-Gateway {
    Write-Host "Stopping api-gateway so Certbot standalone can bind port 80..."
    & docker compose -f $composeFile stop api-gateway
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to stop api-gateway"
    }
}

function Start-Gateway {
    Write-Host "Starting api-gateway..."
    & docker compose -f $composeFile up -d api-gateway
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to start api-gateway"
    }
}

function Copy-EnvoyCertificates {
    param([string]$CertDomain)

    $liveDir = Join-Path $certbotConfigDir ("live\" + $CertDomain)
    $fullchainSource = Join-Path $liveDir "fullchain.pem"
    $privkeySource = Join-Path $liveDir "privkey.pem"
    $fullchainTarget = Join-Path $envoyCertDir "fullchain.pem"
    $privkeyTarget = Join-Path $envoyCertDir "privkey.pem"

    if (-not (Test-Path -LiteralPath $fullchainSource)) {
        throw "Missing certificate file: $fullchainSource"
    }
    if (-not (Test-Path -LiteralPath $privkeySource)) {
        throw "Missing private key file: $privkeySource"
    }

    Copy-Item -LiteralPath $fullchainSource -Destination $fullchainTarget -Force
    Copy-Item -LiteralPath $privkeySource -Destination $privkeyTarget -Force

    Write-Host "Copied fullchain.pem and privkey.pem into $envoyCertDir"
}

function Get-CertificateDomain {
    $liveRoot = Join-Path $certbotConfigDir "live"
    if (-not (Test-Path -LiteralPath $liveRoot)) {
        throw "No existing Let's Encrypt certificates found in $liveRoot"
    }

    $directories = Get-ChildItem -LiteralPath $liveRoot -Directory | Sort-Object Name
    if ($directories.Count -eq 0) {
        throw "No existing Let's Encrypt certificates found in $liveRoot"
    }
    if ($directories.Count -gt 1) {
        throw "Multiple certificate directories found. Pass -Domain explicitly."
    }

    return $directories[0].Name
}

Ensure-Directory $certbotConfigDir
Ensure-Directory $certbotWorkDir
Ensure-Directory $envoyCertDir

$gatewayWasStopped = $false

try {
    if ($Action -eq "local") {
        if ($RestartGateway) {
            Require-Command "docker"
        }

        Generate-LocalCertificates

        if ($RestartGateway) {
            Start-Gateway
        }
    }
    elseif ($Action -eq "issue") {
        Require-Command "docker"
        if ([string]::IsNullOrWhiteSpace($Domain)) {
            throw "Domain is required for issue"
        }
        Stop-Gateway
        $gatewayWasStopped = $true

        $issueArgs = @(
            "certonly",
            "--standalone",
            "--non-interactive",
            "--agree-tos",
            "--cert-name", $Domain,
            "-d", $Domain
        )

        if ([string]::IsNullOrWhiteSpace($Email)) {
            $issueArgs += "--register-unsafely-without-email"
        }
        else {
            $issueArgs += @("--no-eff-email", "--email", $Email)
        }

        if ($DryRun) {
            $issueArgs += "--dry-run"
        }

        Run-DockerCertbot -CertbotArgs $issueArgs

        if (-not $DryRun) {
            Copy-EnvoyCertificates -CertDomain $Domain
        }
    }
    elseif ($Action -eq "renew") {
        Require-Command "docker"
        $certDomain = if ([string]::IsNullOrWhiteSpace($Domain)) { Get-CertificateDomain } else { $Domain }

        Stop-Gateway
        $gatewayWasStopped = $true

        $renewArgs = @("renew")
        if ($DryRun) {
            $renewArgs += "--dry-run"
        }

        Run-DockerCertbot -CertbotArgs $renewArgs

        if (-not $DryRun) {
            Copy-EnvoyCertificates -CertDomain $certDomain
        }
    }

    if ($gatewayWasStopped -or $RestartGateway) {
        Start-Gateway
        $gatewayWasStopped = $false
    }

    Write-Host "Done."
}
catch {
    if ($gatewayWasStopped) {
        try {
            Start-Gateway
        }
        catch {
            Write-Warning "Failed to restart api-gateway automatically: $($_.Exception.Message)"
        }
    }

    throw
}

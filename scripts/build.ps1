Param(
    [string] $Targets = "windows-amd64,windows-arm64",
    [string] $BinDir = "bin",
    [string] $Version = $env:VERSION,
    [string] $Commit = $env:COMMIT,
    [string] $BuildDate = $env:BUILD_DATE
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-CurrentGoPlatform {
    $os = ""
    if ([System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)) {
        $os = "windows"
    }

    $arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture) {
        "X64" { "amd64" }
        "X86" { "386" }
        "Arm64" { "arm64" }
        "Arm" { "arm" }
        default { "" }
    }

    @{ OS = $os; Arch = $arch }
}

$currentPlatform = Get-CurrentGoPlatform

$ldParts = @("-s", "-w")
if ($Version) { $ldParts += "-X github-hub/internal/version.Version=$Version" }
if ($Commit) { $ldParts += "-X github-hub/internal/version.Commit=$Commit" }
if ($BuildDate) { $ldParts += "-X github-hub/internal/version.BuildDate=$BuildDate" }
$ldArgs = @()
if ($ldParts.Count -gt 0) {
    $ldArgs = @("-ldflags", ($ldParts -join " "))
}

function Invoke-BuildPair {
    param(
        [Parameter(Mandatory)] [string] $OS,
        [Parameter(Mandatory)] [string] $Arch
    )

    if ($OS -ne "windows") {
        Write-Error "Only windows targets are supported (got $OS-$Arch)." -ErrorAction Stop
    }

    $suffix = ".exe"
    $outDir = Join-Path $BinDir "$OS-$Arch"
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

    Write-Host "Building $OS-$Arch ..."
    $env:GOOS = $OS
    $env:GOARCH = $Arch
    $env:CGO_ENABLED = "0"

    go build -trimpath @ldArgs -o (Join-Path $outDir "ghh$suffix") ./cmd/ghh
    go build -trimpath @ldArgs -o (Join-Path $outDir "ghh-server$suffix") ./cmd/ghh-server

    if ($currentPlatform.OS -and $currentPlatform.Arch -and $OS -eq $currentPlatform.OS -and $Arch -eq $currentPlatform.Arch) {
        Write-Host "Copying $OS-$Arch build to $BinDir/ ..."
        Copy-Item -Force (Join-Path $outDir "ghh$suffix") (Join-Path $BinDir "ghh$suffix")
        Copy-Item -Force (Join-Path $outDir "ghh-server$suffix") (Join-Path $BinDir "ghh-server$suffix")
    }
}

$list = $Targets -split "," | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" }
if ($list.Count -eq 0) {
    Write-Error "No targets specified."
}

foreach ($t in $list) {
    $parts = $t -split "-"
    if ($parts.Count -ne 2) {
        Write-Error "Invalid target format: $t (expected os-arch, e.g., windows-amd64)"
    }
    Invoke-BuildPair -OS $parts[0] -Arch $parts[1]
}

Write-Host "Done."


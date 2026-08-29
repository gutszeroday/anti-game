<#
build.ps1 - antigame'i dagitilabilir halde derler.

Bu dosya bilerek yalnizca ASCII karakter icerir: Windows PowerShell 5.1,
BOM tasimayan bir .ps1'i ANSI kod sayfasiyla okur ve Turkce harfler
ayristirma hatasina yol acar.

Neden betik: -H=windowsgui bayragi unutulursa cift tiklamada arkada
siyah bir konsol penceresi acilir. Bayrak eskiden yalnizca plan
dosyalarinda yaziyordu; orada durdugu surece her derlemede yeniden
kaybolabilir.

Neden uc mimari: "This app can't run on your PC" hatasi PE dosyasinin
makine tipi ile isletim sisteminin uyusmadigini soyler. amd64 exe, ARM
veya 32-bit bir Windows'ta bu hatayi verir. Uc kopya da ureterek
klasoru oldugu gibi tasimak yetiyor.

Kullanim:
  .\build.ps1              Ucunu de derler, dist\ klasorunu hazirlar
  .\build.ps1 -Arch amd64  Yalnizca bir mimari
  .\build.ps1 -Test        Once testleri calistirir
#>
param(
    [ValidateSet('amd64', 'arm64', '386', 'all')]
    [string]$Arch = 'all',
    [switch]$Test
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$dist = Join-Path $root 'dist'

# Kullanicinin gordugu ad ile Go'nun mimari adi ayni degil: dosya
# adinda "x64" okunur, GOARCH'ta "amd64" gecerlidir.
$targets = @(
    @{ goarch = 'amd64'; name = 'antigame-x64.exe';   machine = 0x8664 }
    @{ goarch = 'arm64'; name = 'antigame-arm64.exe'; machine = 0xAA64 }
    @{ goarch = '386';   name = 'antigame-x86.exe';   machine = 0x014C }
)
if ($Arch -ne 'all') {
    $targets = $targets | Where-Object { $_.goarch -eq $Arch }
}

if ($Test) {
    Write-Host 'Testler calisiyor...' -ForegroundColor Cyan
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw "testler basarisiz (cikis $LASTEXITCODE)" }
}

if (-not (Test-Path $dist)) { New-Item -ItemType Directory -Force $dist | Out-Null }

# PE basligindan uc seyi dogruluyoruz: makine tipi hedefe uyuyor mu,
# alt sistem GUI mi (konsol penceresi acilmayacak mi) ve manifest
# gomulu mu. Ucu de derleme sonrasi sessizce yanlis olabilir.
function Test-PEHeader {
    param([string]$Path, [int]$ExpectMachine)

    $bytes = [System.IO.File]::ReadAllBytes($Path)
    $peOffset = [BitConverter]::ToInt32($bytes, 0x3C)
    $machine = [BitConverter]::ToUInt16($bytes, $peOffset + 4)
    $subsystem = [BitConverter]::ToUInt16($bytes, $peOffset + 0x5C)
    $text = [System.Text.Encoding]::ASCII.GetString($bytes)

    $problems = @()
    if ($machine -ne $ExpectMachine) {
        $problems += ('makine tipi 0x{0:x} bekleniyordu, 0x{1:x} cikti' -f $ExpectMachine, $machine)
    }
    if ($subsystem -ne 2) {
        $problems += ('alt sistem {0}, 2 (GUI) olmali: -H=windowsgui kayip, cift tikta konsol acilacak' -f $subsystem)
    }
    if ($text -notmatch 'requestedExecutionLevel') {
        $problems += 'manifest gomulmemis: DPI ve gorsel stiller bozuk olacak'
    }
    return $problems
}

foreach ($t in $targets) {
    $out = Join-Path $dist $t.name
    Write-Host ('Derleniyor: {0} ({1})' -f $t.name, $t.goarch) -ForegroundColor Cyan

    $env:GOOS = 'windows'
    $env:GOARCH = $t.goarch
    # -s -w: hata ayiklama tablolarini atar, dosyayi kucultur.
    # -H=windowsgui: alt sistemi GUI yapar; konsol penceresi acilmaz.
    & go build -ldflags '-s -w -H=windowsgui' -o $out ./cmd/antigame
    $code = $LASTEXITCODE
    Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
    if ($code -ne 0) { throw ('{0} derlenemedi (cikis {1})' -f $t.name, $code) }

    $problems = Test-PEHeader -Path $out -ExpectMachine $t.machine
    if ($problems.Count -gt 0) {
        foreach ($p in $problems) { Write-Host "  HATA: $p" -ForegroundColor Red }
        throw ('{0} dogrulamayi gecemedi' -f $t.name)
    }
    $kb = [math]::Round((Get-Item $out).Length / 1KB)
    Write-Host ('  tamam: {0} KB, GUI alt sistemi, manifest gomulu' -f $kb) -ForegroundColor Green
}

if ($Arch -eq 'all') {
    Copy-Item (Join-Path $root 'packaging\BASLAT.cmd') $dist -Force
    Copy-Item (Join-Path $root 'packaging\OKU-BENI.txt') $dist -Force
    Write-Host ''
    Write-Host ('Dagitim klasoru hazir: {0}' -f $dist) -ForegroundColor Green
    Write-Host 'Klasorun tamamini karsi bilgisayara kopyalayip BASLAT.cmd calistirin.'
}

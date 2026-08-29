@echo off
rem antigame baslatici. Bilgisayarin mimarisine uyan kopyayi calistirir.
rem
rem Yanlis mimarideki exe "This app can't run on your PC" hatasi verir;
rem kullanicinin hangisini secmesi gerektigini bilmesi gerekmesin diye
rem secim burada yapiliyor. Bu dosya sadece secip cikar, arka planda
rem kalmaz, bellek tuketmez.

setlocal

rem 32-bit bir kabuk 64-bit Windows uzerinde calisiyorsa gercek mimari
rem PROCESSOR_ARCHITECTURE'da degil PROCESSOR_ARCHITEW6432'de yazar.
set "ARCH=%PROCESSOR_ARCHITECTURE%"
if defined PROCESSOR_ARCHITEW6432 set "ARCH=%PROCESSOR_ARCHITEW6432%"

if /i "%ARCH%"=="ARM64" set "EXE=antigame-arm64.exe"
if /i "%ARCH%"=="AMD64" set "EXE=antigame-x64.exe"
if /i "%ARCH%"=="x86"   set "EXE=antigame-x86.exe"

if not defined EXE (
  echo Bilinmeyen islemci mimarisi: %ARCH%
  echo 32-bit kopya deneniyor...
  set "EXE=antigame-x86.exe"
)

if not exist "%~dp0%EXE%" (
  echo Eksik dosya: %EXE%
  echo Klasorun tamamini kopyaladiginizdan emin olun.
  pause
  exit /b 1
)

start "" "%~dp0%EXE%"
exit /b 0

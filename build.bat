@echo off
chcp 65001 >nul
echo Compiling IDM Auto Closer...

REM Check if Go is installed
go version >nul 2>&1
if %errorlevel% neq 0 (
    echo Error: Go is not installed or not in PATH.
    echo Please install Go from https://go.dev/dl/
    exit /b 1
)

REM Read the release version from main.go so the output name and binary agree.
REM The multiline pattern ignores commented-out lines and allows flexible whitespace.
for /f "delims=" %%V in ('powershell -NoProfile -ExecutionPolicy Bypass -Command "$content=Get-Content -Raw .\main.go; $pattern='(?m)^[\t ]*const[\t ]+appVersion[\t\r\n ]*=[\t\r\n ]*'+[char]34+'([^'+[char]34+']+)'+[char]34; if ($content -match $pattern) { $matches[1] } else { exit 1 }"') do set "VERSION=%%V"
if not defined VERSION (
    echo Error: Could not read appVersion from main.go.
    exit /b 1
)

set "OUTPUT=IDM_Auto_Closer-Windows-%VERSION%.exe"

if exist "%OUTPUT%" (
    echo Removing previous build: %OUTPUT%
    del /f /q "%OUTPUT%"
    if exist "%OUTPUT%" (
        echo Error: Could not remove previous build.
        exit /b 1
    )
)

REM Local builds keep the console visible. CI passes "release" for the GUI build.
set "LDFLAGS=-X main.buildVersion=%VERSION%"
if /I "%~1"=="release" set "LDFLAGS=-H=windowsgui -X main.buildVersion=%VERSION%"

echo Building...
go build -ldflags "%LDFLAGS%" -o "%OUTPUT%" .

if %errorlevel% equ 0 (
    echo Compilation successful!
    echo Created %OUTPUT%
) else (
    echo Compilation failed.
    exit /b 1
)
exit /b 0

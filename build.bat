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

set "OUTPUT_DIR=IDM_Auto_Closer"
set "MAIN_OUTPUT=%OUTPUT_DIR%\IDM_Auto_Closer-Windows-%VERSION%.exe"
set "CLEANUP_OUTPUT=%OUTPUT_DIR%\Data Cleanup.exe"

if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"

for %%F in ("%MAIN_OUTPUT%" "%CLEANUP_OUTPUT%" "%OUTPUT_DIR%\IDM_Auto_Closer-Cleanup-Windows-*.exe") do (
    if exist "%%~F" (
        echo Removing previous build: %%~F
        del /f /q "%%~F"
        if exist "%%~F" (
            echo Error: Could not remove previous build: %%~F
            exit /b 1
        )
    )
)

if not exist "%OUTPUT_DIR%" (
    echo Error: Could not create output directory.
    exit /b 1
)

REM Both programs run as Windows GUI applications, so no terminal is shown.
set "LDFLAGS=-H=windowsgui -X main.buildVersion=%VERSION%"
set "CLEANUP_LDFLAGS=-H=windowsgui"

echo Building main program...
go build -ldflags "%LDFLAGS%" -o "%MAIN_OUTPUT%" .
if %errorlevel% neq 0 (
    echo Main program compilation failed.
    exit /b 1
)

echo Building cleanup program...
go build -ldflags "%CLEANUP_LDFLAGS%" -o "%CLEANUP_OUTPUT%" .\cleanup
if %errorlevel% neq 0 (
    echo Cleanup program compilation failed.
    exit /b 1
)

if not exist "%MAIN_OUTPUT%" (
    echo Error: Main program output was not created.
    exit /b 1
)
if not exist "%CLEANUP_OUTPUT%" (
    echo Error: Cleanup program output was not created.
    exit /b 1
)

echo Compilation successful!
echo Created %MAIN_OUTPUT%
echo Created %CLEANUP_OUTPUT%
exit /b 0

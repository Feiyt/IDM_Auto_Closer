@echo off
chcp 65001 >nul
echo Compiling IDM Auto Closer...

REM Check if Go is installed
go version >nul 2>&1
if %errorlevel% neq 0 (
    echo Error: Go is not installed or not in PATH.
    echo Please install Go from https://go.dev/dl/
    pause
    exit /b 1
)

REM Initialize Go module if not exists
if not exist go.mod (
    echo Initializing Go module...
    go mod init IDM_Auto_Closer
    go mod tidy
)

REM Build with GUI flag to hide console window
REM To see the console output for debugging, use: go build -o IDM_Auto_Closer.exe main.go
echo Building...
go build -ldflags "-H=windowsgui" -o IDM_Auto_Closer.exe main.go

if %errorlevel% equ 0 (
    echo Compilation successful!
    echo Created IDM_Auto_Closer.exe
) else (
    echo Compilation failed.
)

pause

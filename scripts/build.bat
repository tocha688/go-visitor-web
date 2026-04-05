@echo off
set APP_NAME=visitor
set VERSION=1.0.0

echo Building %APP_NAME% v%VERSION% for multiple platforms...

if not exist dist mkdir dist

echo Building for Windows (amd64)...
go build -ldflags="-s -w" -o dist\%APP_NAME%-windows-amd64.exe .

echo Building for Linux (amd64)...
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o dist\%APP_NAME%-linux-amd64 .

echo Build complete. Output files:
dir /b dist

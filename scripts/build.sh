#!/bin/bash

APP_NAME="visitor"
VERSION="1.0.0"

echo "Building $APP_NAME v$VERSION for multiple platforms..."

mkdir -p dist

echo ">>> Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/${APP_NAME}-windows-amd64.exe .
echo ">>> Building for Windows (386)..."
GOOS=windows GOARCH=386 go build -ldflags="-s -w" -o dist/${APP_NAME}-windows-386.exe .

echo ">>> Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/${APP_NAME}-linux-amd64 .
echo ">>> Building for Linux (386)..."
GOOS=linux GOARCH=386 go build -ldflags="-s -w" -o dist/${APP_NAME}-linux-386 .

echo ">>> Building for Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/${APP_NAME}-linux-arm64 .

echo ""
echo "Build complete. Output files:"
ls -lh dist/

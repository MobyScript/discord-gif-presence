#!/bin/sh
# Builds the Windows app into dist/. Run on any OS that has Go installed.
set -e
cd "$(dirname "$0")"
go test ./...
GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w" -o dist/DiscordGifPresence.exe .
echo "Built dist/DiscordGifPresence.exe"

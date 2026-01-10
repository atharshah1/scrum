#!/usr/bin/env bash
set -e

mkdir -p dist

GOOS=linux   GOARCH=amd64 go build -o dist/scrum-linux
GOOS=darwin  GOARCH=arm64 go build -o dist/scrum-macos
GOOS=windows GOARCH=amd64 go build -o dist/scrum.exe

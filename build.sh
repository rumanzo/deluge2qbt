#!/bin/bash
GOOS=windows GOACH=amd64 go build -o deluge2qbt_v${1}_amd64.exe
GOOS=windows GOARCH=386 go build -o deluge2qbt_v${1}_i386.exe
GOOS=linux GOARCH=amd64 go build -o deluge2qbt_v${1}_amd64_linux
GOOS=linux GOARCH=386 go build -o deluge2qbt_v${1}_i386_linux
GOOS=linux GOARCH=arm go build -o deluge2qbt_v${1}_arm_linux
GOOS=linux GOARCH=arm64 go build -o deluge2qbt_v${1}_arm64_linux
GOOS=darwin GOARCH=amd64 go build -o deluge2qbt_v${1}_amd64_macos
GOOS=darwin GOARCH=386 go build -o deluge2qbt_v${1}_i386_macos
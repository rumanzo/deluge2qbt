gotag=1.18.1-bullseye

commit=$(shell git rev-parse HEAD)

dockercmd=docker run --rm -v $(CURDIR):/usr/src/deluge2qbt -w /usr/src/deluge2qbt
buildtags = -tags forceposix
buildenvs = -e CGO_ENABLED=0
version = 1.999
ldflags = -ldflags="-X 'main.version=$(version)' -X 'main.commit=$(commit)' -X 'main.buildImage=golang:$(gotag)'"

all: | build


build: windows linux darwin

windows:
	$(dockercmd) $(buildenvs) -e GOOS=windows -e GOARCH=amd64 golang:$(gotag) go build -v $(buildtags) $(ldflags) -o deluge2qbt_v$(version)_amd64.exe
	$(dockercmd) $(buildenvs) -e GOOS=windows -e GOARCH=386 golang:$(gotag) go build -v $(buildtags) $(ldflags) -o deluge2qbt_v$(version)_i386.exe

linux:
	$(dockercmd) $(buildenvs) -e GOOS=linux -e GOARCH=amd64 golang:$(gotag) go build -v $(buildtags) $(ldflags) -o deluge2qbt_v$(version)_amd64_linux
	$(dockercmd) $(buildenvs) -e GOOS=linux -e GOARCH=386 golang:$(gotag) go build -v $(buildtags) $(ldflags) -o deluge2qbt_v$(version)_i386_linux

darwin:
	$(dockercmd) $(buildenvs) -e GOOS=darwin -e GOARCH=amd64 golang:$(gotag) go build -v $(buildtags) $(ldflags) -o deluge2qbt_v$(version)_amd64_macos
	$(dockercmd) $(buildenvs) -e GOOS=darwin -e GOARCH=arm64 golang:$(gotag) go build -v $(buildtags) $(ldflags) -o deluge2qbt_v$(version)_arm64_macos
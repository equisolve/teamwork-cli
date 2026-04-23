BINARY=teamwork
ALIAS=tw
VERSION?=0.1.0

.PHONY: build install uninstall clean all test

build:
	go build -o $(BINARY) -ldflags "-s -w" .

install: build
	cp $(BINARY) /usr/local/bin/$(BINARY)
	ln -sf /usr/local/bin/$(BINARY) /usr/local/bin/$(ALIAS)

uninstall:
	rm -f /usr/local/bin/$(BINARY) /usr/local/bin/$(ALIAS)

all: build-darwin-arm64 build-darwin-amd64 build-linux-amd64

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY)-darwin-arm64 -ldflags "-s -w" .

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY)-darwin-amd64 -ldflags "-s -w" .

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)-linux-amd64 -ldflags "-s -w" .

clean:
	rm -f $(BINARY) $(BINARY)-*

test:
	go test ./...

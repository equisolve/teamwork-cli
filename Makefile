BINARY=teamwork
ALIAS=tw
VERSION?=0.3.1
LDFLAGS=-s -w -X github.com/equisolve/teamwork-cli/cmd.version=$(VERSION)

.PHONY: build install uninstall clean all test

build:
	go build -o $(BINARY) -ldflags "$(LDFLAGS)" .

install: build
	cp $(BINARY) /usr/local/bin/$(BINARY)
	ln -sf /usr/local/bin/$(BINARY) /usr/local/bin/$(ALIAS)

uninstall:
	rm -f /usr/local/bin/$(BINARY) /usr/local/bin/$(ALIAS)

all: build-darwin-arm64 build-darwin-amd64 build-linux-amd64

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY)-darwin-arm64 -ldflags "$(LDFLAGS)" .

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY)-darwin-amd64 -ldflags "$(LDFLAGS)" .

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)-linux-amd64 -ldflags "$(LDFLAGS)" .

clean:
	rm -f $(BINARY) $(BINARY)-*

test:
	go test ./...

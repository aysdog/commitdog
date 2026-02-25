BINARY=commitdog
VERSION=0.1.0

build:
	go build -ldflags="-s -w" -o $(BINARY) .

install:
	go install -ldflags="-s -w" .

# cross-compile all platforms
release:
	mkdir -p dist
	GOOS=linux   GOARCH=amd64  go build -ldflags="-s -w" -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64  go build -ldflags="-s -w" -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64  go build -ldflags="-s -w" -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64  go build -ldflags="-s -w" -o dist/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64  go build -ldflags="-s -w" -o dist/$(BINARY)-windows-amd64.exe .

clean:
	rm -f $(BINARY)
	rm -rf dist/

test:
	go test ./...

vet:
	go vet ./...

.PHONY: build install release clean test vet
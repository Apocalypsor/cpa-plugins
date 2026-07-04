.PHONY: test build clean

test:
	go test ./...

build:
	go build -buildmode=c-shared -o codex-credentials.so .

clean:
	rm -f codex-credentials.so codex-credentials.h

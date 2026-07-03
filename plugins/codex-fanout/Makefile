UNAME_S := $(shell uname -s)
ifeq ($(OS),Windows_NT)
EXT := dll
else ifeq ($(UNAME_S),Darwin)
EXT := dylib
else
EXT := so
endif

.PHONY: test build clean

test:
	go test ./...

build:
	go build -buildmode=c-shared -o codex-fanout.$(EXT) .
	rm -f codex-fanout.h

clean:
	rm -f codex-fanout.dylib codex-fanout.so codex-fanout.dll codex-fanout.h

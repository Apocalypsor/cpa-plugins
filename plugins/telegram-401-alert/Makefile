.PHONY: test build clean

test:
	go test ./...

build:
	go build -buildmode=c-shared -o telegram-401-alert.so .

clean:
	rm -f telegram-401-alert.so telegram-401-alert.h

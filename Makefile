.PHONY: all server discover clean test test-discover check-size

all: server

server:
	go build -o server ./cmd/server

discover:
	go build -tags xlsx_discover -o xlsx-discover ./tools/xlsx-discover/

clean:
	rm -f server xlsx-discover

test:
	go test ./...

test-discover:
	go test -tags xlsx_discover ./tools/xlsx-discover/ -v -count=1

check-size: server
	@size=$$(stat -c%s server 2>/dev/null || stat -f%z server); \
	echo "Server binary size: $$size bytes"

VERSION ?= dev
HOST_ARCH := $(shell go env GOARCH)

.PHONY: all server discover clean test test-discover check-size docker docker-run

all: server

server:
	go build -o server ./cmd/server

discover:
	go build -tags xlsx_discover -o xlsx-discover ./tools/xlsx-discover/

clean:
	rm -f server xlsx-discover
	rm -rf dist/

test:
	go test ./...

test-discover:
	go test -tags xlsx_discover ./tools/xlsx-discover/ -v -count=1

check-size: server
	@size=$$(stat -c%s server 2>/dev/null || stat -f%z server); \
	echo "Server binary size: $$size bytes"

# Cross-compile a linux binary for any GOARCH. Used by the docker target
# and by the release workflow.
dist/server-%:
	@mkdir -p dist
	GOOS=linux GOARCH=$* CGO_ENABLED=0 go build \
		-ldflags="-s -w -X main.version=$(VERSION)" \
		-o $@ ./cmd/server

docker: dist/server-$(HOST_ARCH)
	docker build -t sofar-hyd-diag:$(VERSION) .

docker-run: docker
	docker run --rm -p 8080:8080 \
		-e INVERTER_HOST=10.5.99.29 \
		-e INVERTER_PORT=4192 \
		sofar-hyd-diag:$(VERSION)

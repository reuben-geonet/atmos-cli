HELPER := bin/atmosctl
GOCACHE ?= /tmp/atmos-cli-gocache
GORELEASER ?= go run github.com/goreleaser/goreleaser/v2@v2.16.0

.PHONY: build clean package release-check test version

build:
	mkdir -p bin $(GOCACHE)
	GOCACHE=$(GOCACHE) go build -buildvcs=false -o $(HELPER) ./cmd/atmosctl

test:
	GOCACHE=$(GOCACHE) go test ./...

version: build
	$(HELPER) version

release-check:
	$(GORELEASER) check

package:
	$(GORELEASER) release --snapshot --clean

clean:
	rm -rf bin dist

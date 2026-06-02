HELPER := bin/atmosctl
GOCACHE ?= /tmp/atmos-cli-gocache

.PHONY: build clean test version

build:
	mkdir -p bin $(GOCACHE)
	GOCACHE=$(GOCACHE) go build -buildvcs=false -o $(HELPER) ./cmd/atmosctl

test:
	GOCACHE=$(GOCACHE) go test ./...

version: build
	$(HELPER) version

clean:
	rm -rf bin

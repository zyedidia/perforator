VERSION = $(shell GOOS=$(shell go env GOHOSTOS) GOARCH=$(shell go env GOHOSTARCH) \
	go run tools/build-version.go)
GOVARS = -X main.Version=$(VERSION)

build:
	go build -trimpath -ldflags "-s -w $(GOVARS)" ./cmd/perforator
install:
	go install -trimpath -ldflags "-s -w $(GOVARS)" ./cmd/perforator
clean:
	rm -f perforator

.PHONY: build clean install

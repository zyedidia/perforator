VERSION = $(shell GOOS=$(shell go env GOHOSTOS) GOARCH=$(shell go env GOHOSTARCH) \
	go run tools/build-version.go)
GOVARS = -X main.Version=$(VERSION)

build:
	go build -trimpath -ldflags "-s -w $(GOVARS)" ./cmd/perforator

install:
	go install -trimpath -ldflags "-s -w $(GOVARS)" ./cmd/perforator

perforator.1: man/perforator.md
	pandoc man/perforator.md -s -t man -o perforator.1

package: build perforator.1
	mkdir perforator-$(VERSION)
	cp README.md perforator-$(VERSION)
	cp LICENSE perforator-$(VERSION)
	cp perforator.1 perforator-$(VERSION)
	cp perforator perforator-$(VERSION)
	tar -czf perforator-$(VERSION).tar.gz perforator-$(VERSION)

clean:
	rm -f perforator perforator.1 perforator-*.tar.gz
	rm -rf perforator-*/

.PHONY: build clean install package

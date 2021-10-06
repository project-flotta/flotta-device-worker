VERSION = 1.0
RELEASE = 1
DIST_DIR = $(shell pwd)/dist

OS :=$(shell awk -F= '/^ID/{print $$2}' /etc/os-release)

ifeq ($(OS),fedora)
	LIBEXECDIR ?= /usr/local/libexec
else
	LIBEXECDIR ?= /usr/libexec
endif

export GOFLAGS=-mod=vendor -tags=containers_image_openpgp

test-tools:
ifeq (, $(shell which ginkgo))
	go get github.com/onsi/ginkgo/ginkgo
endif


test: test-tools
	ginkgo -r ./internal/* ./cmd/*

generate:
	$(Q) go generate ./...

vendor:
	go mod tidy
	go mod vendor

build:
	mkdir -p ./bin
	CGO_ENABLED=0 go build -o ./bin ./cmd/device-worker

build-arm64:
	mkdir -p ./bin
	GOARCH=arm64 CGO_ENABLED=0 go build -o ./bin/device-worker-aarch64 ./cmd/device-worker

install: build
	sudo install -D -m 755 ./bin/device-worker $(LIBEXECDIR)/yggdrasil/device-worker

install-arm64: build-arm64
	sudo install -D -m 755 ./bin/device-worker-aarch64 $(LIBEXECDIR)/yggdrasil/device-worker

clean:
	go mod tidy
	rm -rf bin

rpm:
	install -D -m 755 ./bin/device-worker dist/$(LIBEXECDIR)/yggdrasil/device-worker
	rpmbuild -bb -D "VERSION $(VERSION)" -D "RELEASE $(RELEASE)" -D "_libexecdir $(LIBEXECDIR)" --buildroot $(DIST_DIR) ./k4e-agent.spec

rpm-arm64:
	install -D -m 755 ./bin/device-worker-aarch64 dist/$(LIBEXECDIR)/yggdrasil/device-worker
	rpmbuild -bb -D "VERSION $(VERSION)" -D "RELEASE $(RELEASE)" -D "_libexecdir $(LIBEXECDIR)" --target aarch64 --buildroot $(DIST_DIR) ./k4e-agent.spec

dist: build build-arm64 rpm rpm-arm64

OS :=$(shell awk -F= '/^ID/{print $$2}' /etc/os-release)

ifeq ($(OS),fedora)
	LIBEXECDIR ?= /usr/local/libexec
else
	LIBEXECDIR ?= /usr/libexec
endif

export GOFLAGS=-mod=vendor -tags=containers_image_openpgp

test-tools:
	go get github.com/onsi/ginkgo/ginkgo

test: test-tools
	ginkgo ./internal/* ./cmd/*

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

rpm: VERSION = 1.0
rpm: RELEASE = 1
rpm: RPM_BUILDROOT = $(shell rpmbuild -D "NAME k4e-agent" -D "VERSION $(VERSION)" -D "RELEASE $(RELEASE)" -E %buildroot)
rpm:
	install -D -m 755 ./bin/device-worker $(RPM_BUILDROOT)/$(LIBEXECDIR)/yggdrasil/device-worker
	rpmbuild -bb -D "VERSION $(VERSION)" -D "RELEASE $(RELEASE)" -D "_libexecdir $(LIBEXECDIR)" ./k4e-agent.spec
	rm -rf $(RPM_BUILDROOT)

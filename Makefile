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
	GO111MODULE=off go get github.com/onsi/ginkgo/ginkgo
endif
ifeq (, $(shell which gover))
	GO111MODULE=off go get github.com/sozorogami/gover
endif

test: test-tools
	ginkgo -r $(GINKGO_OPTIONS) ./internal/* ./cmd/*

test-coverage:
test-coverage: GINKGO_OPTIONS ?= --cover
test-coverage: test
	gover
	go tool cover -html gover.coverprofile

test-coverage-clean:
	git ls-files --others --ignored --exclude-standard | grep "coverprofile$$" | xargs rm

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

rpm-tarball:
	 (git archive --prefix k4e-agent-$(VERSION)/ HEAD ) \
	    | gzip > k4e-agent-$(VERSION).tar.gz

rpm-src:
	cp k4e-agent-$(VERSION).tar.gz $(HOME)/rpmbuild/SOURCES/
	rpmbuild -bs \
		-D "VERSION $(VERSION)" \
		-D "RELEASE $(RELEASE)" \
		-D "_libexecdir $(LIBEXECDIR)" \
		--buildroot $(DIST_DIR) ./k4e-agent.spec

rpm-copr: rpm-src
	copr build eloyocoto/k4e-test $(HOME)/rpmbuild/SRPMS/k4e-agent-$(VERSION)-$(RELEASE).*.src.rpm

rpm-build: rpm-src
	rpmbuild $(RPMBUILD_OPTS) --rebuild $(HOME)/rpmbuild/SRPMS/k4e-agent-$(VERSION)-$(RELEASE).*.src.rpm

rpm: rpm-build

rpm-arm64: RPMBUILD_OPTS=--target=aarch64
rpm-arm64: rpm-build

dist: build build-arm64 rpm rpm-arm64

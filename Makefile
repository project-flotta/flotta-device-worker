VERSION = 1.0
RELEASE = 1
DIST_DIR = $(shell pwd)/dist
CGO_ENABLED = 0
OS :=$(shell awk -F= '/^ID/{print $$2}' /etc/os-release)
BUILDROOT ?=

DOCKER ?= podman

ifeq ($(OS),fedora)
	LIBEXECDIR ?= /usr/local/libexec
	SYSCONFDIR ?= /usr/local/etc
else
	LIBEXECDIR ?= /usr/libexec
	SYSCONFDIR ?= /etc
endif

export GOFLAGS=-mod=vendor -tags=containers_image_openpgp

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


##@ Development

test-tools:
ifeq (, $(shell which ginkgo))
	go get github.com/onsi/ginkgo/ginkgo@v1.16.5
endif
ifeq (, $(shell which gover))
	GO111MODULE=off go get github.com/sozorogami/gover
endif

gosec: ## Run gosec locally
	$(DOCKER) run --rm -it -v $(PWD):/opt/data/:z docker.io/securego/gosec -exclude-generated /opt/data/...

test: ## Run unit test on device worker
test: test-tools
	ginkgo --race -r $(GINKGO_OPTIONS) ./internal/* ./cmd/*

test-coverage:
test-coverage: ## Run test and launch coverage tool
test-coverage: GINKGO_OPTIONS ?= --cover
test-coverage: test
	gover
	go tool cover -html gover.coverprofile

test-coverage-clean:
	git ls-files --others --ignored --exclude-standard | grep "coverprofile$$" | xargs rm

generate: ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(Q) go generate ./...

.PHONY: vendor

vendor:
	go mod tidy
	go mod vendor

clean: ## Clean project
	go mod tidy
	rm -rf bin

##@ Build

build-debug: ## Build with race conditions and lock checker
build-debug: BUILD_OPTIONS=--race
build-debug: CGO_ENABLED=1
build-debug: build

build: ## Build device worker
	mkdir -p ./bin
	CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_OPTIONS) -o ./bin ./cmd/device-worker

build-arm64: ## Build device worker for arm64
	mkdir -p ./bin
	GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) go build -o ./bin/device-worker-aarch64 ./cmd/device-worker

##@ Deployment

install-worker-config:
	mkdir -p $(BUILDROOT)$(SYSCONFDIR)/yggdrasil/workers/
	sed 's,#LIBEXEC#,$(LIBEXECDIR),g' config/device-worker.toml > $(BUILDROOT)$(SYSCONFDIR)/yggdrasil/workers/device-worker.toml

install: ## Install device-worker with debug enabled
install-debug: build-debug install-worker-config
	sudo install -D -m 755 ./bin/device-worker $(LIBEXECDIR)/yggdrasil/device-worker

install: ## Install device-worker
install: build install-worker-config
	sudo install -D -m 755 ./bin/device-worker $(LIBEXECDIR)/yggdrasil/device-worker

install-arm64: ## Install device-worker on arm64.
install-arm64: build-arm64 install-worker-config
	sudo install -D -m 755 ./bin/device-worker-aarch64 $(LIBEXECDIR)/yggdrasil/device-worker


rpm-tarball:
	 (git archive --prefix flotta-agent-$(VERSION)/ HEAD ) \
	    | gzip > flotta-agent-$(VERSION).tar.gz

rpm-src: rpm-tarball
	install -D -m644 flotta-agent-$(VERSION).tar.gz --target-directory `rpmbuild -E %_sourcedir`
	rpmbuild -bs \
		-D "VERSION $(VERSION)" \
		-D "RELEASE $(RELEASE)" \
		-D "_libexecdir $(LIBEXECDIR)" \
		--buildroot $(DIST_DIR) ./flotta-agent.spec

rpm-copr: rpm-src
	copr build eloyocoto/k4e-test $(HOME)/rpmbuild/SRPMS/flotta-agent-$(VERSION)-$(RELEASE).*.src.rpm

rpm-build: rpm-src
	rpmbuild $(RPMBUILD_OPTS) --rebuild $(HOME)/rpmbuild/SRPMS/flotta-agent-$(VERSION)-$(RELEASE).*.src.rpm

rpm: ## Create rpm build
	RPMBUILD_OPTS=--target=x86_64 $(MAKE) rpm-build

rpm-arm64: ## Create rpm build for arm64
	RPMBUILD_OPTS=--target=aarch64  $(MAKE) rpm-build

dist: ## Create distribution packages
dist: build build-arm64 rpm rpm-arm64

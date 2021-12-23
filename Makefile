VERSION = 1.0
RELEASE = 1
DIST_DIR = $(shell pwd)/dist
CGO_ENABLED = 0
OS :=$(shell awk -F= '/^ID/{print $$2}' /etc/os-release)

ifeq ($(OS),fedora)
	LIBEXECDIR ?= /usr/local/libexec
else
	LIBEXECDIR ?= /usr/libexec
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
	GO111MODULE=off go get github.com/onsi/ginkgo/ginkgo
endif
ifeq (, $(shell which gover))
	GO111MODULE=off go get github.com/sozorogami/gover
endif

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

install: ## Install device-worker with debug enabled
install-debug: build-debug
	sudo install -D -m 755 ./bin/device-worker $(LIBEXECDIR)/yggdrasil/device-worker

install: ## Install device-worker
install: build
	sudo install -D -m 755 ./bin/device-worker $(LIBEXECDIR)/yggdrasil/device-worker

install-arm64: ## Install device-worker on arm64.
install-arm64: build-arm64
	sudo install -D -m 755 ./bin/device-worker-aarch64 $(LIBEXECDIR)/yggdrasil/device-worker


rpm-tarball:
	 (git archive --prefix k4e-agent-$(VERSION)/ HEAD ) \
	    | gzip > k4e-agent-$(VERSION).tar.gz

rpm-src: rpm-tarball
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

rpm: ## Create rpm build
rpm: rpm-build

rpm-arm64: ## Create rpm build for arm64
rpm-arm64: RPMBUILD_OPTS=--target=aarch64
rpm-arm64: rpm-build

dist: ## Create distribution packages
dist: build build-arm64 rpm rpm-arm64

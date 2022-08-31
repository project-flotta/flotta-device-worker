VERSION = 0.2.0
RELEASE = 3
DIST_DIR = $(shell pwd)/dist
CGO_ENABLED = 0
OS :=$(shell awk -F= '/^ID/{print $$2}' /etc/os-release)
BUILDROOT ?=

DOCKER ?= podman
IMG ?= quay.io/project-flotta/edgedevice:latest

# Installation directories
PREFIX        ?= /usr/local
LIBEXECDIR    ?= $(PREFIX)/libexec
SYSCONFDIR    ?= $(PREFIX)/etc

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

GOVER = $(shell pwd)/bin/gover
gover:
ifeq (, $(shell which ginkgo 2> /dev/null))
	$(call go-install-tool,$(GOVER),github.com/sozorogami/gover)
endif

GINKGO = $(shell pwd)/bin/ginkgo
ginkgo:
ifeq (, $(shell which ginkgo 2> /dev/null))
	$(call go-install-tool,$(GINKGO),github.com/onsi/ginkgo/v2/ginkgo@v2.1.3)
else
	GINKGO=$(shell which ginkgo)
endif

GOJSONSCHEMA = $(shell pwd)/bin/gojsonschema
gojsonschema:
ifeq (, $(shell which gojsonschema 2> /dev/null))
	$(call go-install-tool,$(GOJSONSCHEMA),github.com/atombender/go-jsonschema/cmd/gojsonschema)
else
	GOJSONSCHEMA=$(shell which gojsonschema)
endif



test-tools: ## Install test-tools
test-tools: ginkgo gover

gosec: ## Run gosec locally
	$(DOCKER) run --rm -it -v $(PWD):/opt/data/:z docker.io/securego/gosec -exclude-generated /opt/data/...

test: ## Run unit test on device worker
test: test-tools
	$(GINKGO) --race -r $(GINKGO_OPTIONS) --label-filter="!root" ./internal/* ./cmd/*
	sudo $(GINKGO) --race -r $(GINKGO_OPTIONS) --label-filter="root" ./internal/* ./cmd/*

test-coverage:
test-coverage: ## Run test and launch coverage tool
test-coverage: GINKGO_OPTIONS ?= --cover
test-coverage: test
	gover
	go tool cover -html gover.coverprofile

test-coverage-clean:
	git ls-files --others --ignored --exclude-standard | grep "coverprofile$$" | xargs rm

generate-tools: gojsonschema

generate-messages: generate-tools 
	mkdir -p internal/ansible/model/message/
	$(GOJSONSCHEMA) --yaml-extension yaml -p message internal/ansible/schema/ansibleRunnerJobEvent.yaml -o internal/ansible/model/message/runner-job-event-gen.go 

generate: ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(Q) go generate ./...

.PHONY: vendor

vendor:
	go mod tidy
	go mod vendor

bump-operator: ## Bump flotta operator version dependency to the latest main
	$(eval OPARATOR_VERSION := $(shell curl -s https://api.github.com/repos/project-flotta/flotta-operator/commits/main | jq '.sha' -r))
	go get -d github.com/project-flotta/flotta-operator@$(OPARATOR_VERSION)

clean: ## Clean project
	go mod tidy
	rm -rf bin

##@ Build

build-debug: ## Build with race conditions and lock checker
build-debug: BUILD_OPTIONS=--race
build-debug: CGO_ENABLED=1
build-debug: build

build: ## Build device worker
build: generate-messages
	mkdir -p ./bin
	CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_OPTIONS) -o ./bin ./cmd/device-worker

build-arm64: ## Build device worker for arm64
	mkdir -p ./bin
	GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) go build -o ./bin/device-worker-aarch64 ./cmd/device-worker

##@ Deployment

install-worker-config:
	mkdir -p $(BUILDROOT)$(SYSCONFDIR)/yggdrasil/workers/
	mkdir -p $(BUILDROOT)$(SYSCONFDIR)/yggdrasil/device/volumes
	chown $(VOLUME_USER):$(VOLUME_USER) -R $(BUILDROOT)$(SYSCONFDIR)/yggdrasil/device/volumes
	sed 's,#LIBEXEC#,$(LIBEXECDIR),g;s,#USER#,USER=$(USER),g;s,#HOME#,HOME=$(HOME),g;s,#RUNTIME_DIR#,FLOTTA_XDG_RUNTIME_DIR=$(FLOTTA_XDG_RUNTIME_DIR),g' config/device-worker.toml > $(BUILDROOT)$(SYSCONFDIR)/yggdrasil/workers/device-worker.toml

install: ## Install device-worker with debug enabled
install-debug: build-debug needs-root
	$(MAKE) install-worker-config HOME=$(HOME) VOLUME_USER=$(USER) FLOTTA_XDG_RUNTIME_DIR=$(XDG_RUNTIME_DIR)
	install -D -m 755 ./bin/device-worker $(LIBEXECDIR)/yggdrasil/device-worker

install: ## Install device-worker
install: build needs-root
	$(MAKE) install-worker-config HOME=$(HOME) VOLUME_USER=$(USER) FLOTTA_XDG_RUNTIME_DIR=$(XDG_RUNTIME_DIR)
	install -D -m 755 ./bin/device-worker $(LIBEXECDIR)/yggdrasil/device-worker

install-arm64: ## Install device-worker on arm64.
install-arm64: build-arm64 needs-root
	$(MAKE) install-worker-config
	install -D -m 755 ./bin/device-worker-aarch64 $(LIBEXECDIR)/yggdrasil/device-worker

uninstall: needs-root clean
	rm -rf $(SYSCONFDIR)/yggdrasil/device/*
	rm -rf $(LIBEXECDIR)/yggdrasil/device-worker

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

rpm-copr-testing: rpm-src
	copr build project-flotta/flotta-testing $(HOME)/rpmbuild/SRPMS/flotta-agent-$(VERSION)-$(RELEASE).*.src.rpm

rpm-copr: rpm-src
	copr build project-flotta/flotta $(HOME)/rpmbuild/SRPMS/flotta-agent-$(VERSION)-$(RELEASE).*.src.rpm

rpm-build: rpm-src
	rpmbuild $(RPMBUILD_OPTS) --rebuild $(HOME)/rpmbuild/SRPMS/flotta-agent-$(VERSION)-$(RELEASE).*.src.rpm

rpm: ## Create rpm build
	RPMBUILD_OPTS=--target=x86_64 $(MAKE) rpm-build

rpm-arm64: ## Create rpm build for arm64
	RPMBUILD_OPTS=--target=aarch64  $(MAKE) rpm-build

deploy-container-image:
	$(DOCKER) build ./tools/ -t ${IMG}
	$(DOCKER) push ${IMG}

dist: ## Create distribution packages
dist: build build-arm64 rpm rpm-arm64


needs-root:
ifneq ($(shell id -u), 0)
	@echo "You must be root to perform this action."
	exit 1
endif

# go-install-tool will 'go install' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-install-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

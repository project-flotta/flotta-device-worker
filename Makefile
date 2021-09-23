LIBEXECDIR ?= /usr/libexec
build:
	mkdir -p ./bin
	CGO_ENABLED=0 go build -tags containers_image_openpgp -o ./bin ./cmd/device-worker

install: build
	sudo install -D -m 755 ./bin/device-worker $(LIBEXECDIR)/yggdrasil/device-worker

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

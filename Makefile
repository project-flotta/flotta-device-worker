build:
	mkdir -p ./bin
	go build -o ./bin ./cmd/device-worker

install: build
	sudo install -D -m 755 ./bin/device-worker /usr/local/libexec/yggdrasil/device-worker

clean:
	rm -rf bin
# Hacking

Because this project is part of Flotta project, all contributing guidelines are
inherit from [flotta-operator](https://github.com/project-flotta/flotta-operator) project.
Detail information about this project is below.

## System dependencies

Install following packages (Fedora):

- btrfs-progs-devel
- gpgme-devel
- device-mapper-devel

## Building and installing

To build and install `device-worker` for yggdrasil (`/usr/local/libexec/yggdrasil`) run:
`make install`.

### RPM build and install
RPM will be located at ~/rpmbuild/RPMS
```
make build
make rpm
```

## Clean start

To start the device-worker in clean (pairing) mode, make sure that following files are not present before starting
yggdrasil:

- `/var/local/yggdrasil/device/device-config.json`
- `/var/local/yggdrasil/device/manifests/*`

# Running

## Preparations

Generate client certificate (`cert.pem`) and key (`key.pem`) and put them in `/etc/pki/consumer` directory.

## Start yggdrasil

To run yggdrasil configured to communicate with the flotta-operator HTTP API running on localhost:8888 execute in yggdrasil
repo (https://github.com/jakub-dzon/yggdrasil) directory :

```
sudo go run ./cmd/yggd \
  --log-level info \
  --protocol http \
  --path-prefix api/flotta-management/v1 \
  --client-id $(cat /etc/machine-id) \
  --cert-file /etc/pki/consumer/cert.pem \
  --key-file /etc/pki/consumer/key.pem \
  --server localhost:8888
```

Also, the worker config need to be defined in the right location:

```
--> cat /usr/local/etc/yggdrasil/workers/device-worker.toml
exec = "/usr/local/libexec/yggdrasil/device-worker"
protocol = "grpc"
env = []
```

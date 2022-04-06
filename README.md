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

- `/var/local/etc/yggdrasil/device/device-config.json`
- `/var/local/yggdrasil/device/manifests/*`

Or run:
`make uninstall`

# Running

## Preparations

Generate client certificate (`cert.pem`) and key (`key.pem`) and put them in `/etc/pki/consumer` directory.

## Start yggdrasil

To run yggdrasil configured to communicate with the flotta-operator HTTP API running on localhost:8888 execute in yggdrasil
repo (https://github.com/RedHatInsights/yggdrasil) directory :

To install Yggdrasil from upstream repo:

```
git clone git@github.com:RedHatInsights/yggdrasil.git
cd yggdrasil
git checkout cdde836c519ac72302714b5f7ae90e45de7c79a7
export PREFIX=/var/local
make bin
```

Start yggdrasil:
```
sudo ./yggd \
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
$ cat /var/local/etc/yggdrasil/workers/device-worker.toml
exec = "/usr/local/libexec/yggdrasil/device-worker"
protocol = "grpc"
env = []
```

A common error is that worker cannot be started, with the following error
message:

```
[yggd] 2022/02/21 16:55:00 ./cmd/yggd/main.go:189: starting yggd version 0.2.98
cannot stop workers: cannot stop worker: cannot stop worker: cannot stop process: os: process already finished
```

To fix that error, you need to run the following, because device-worker was
already stopped:

```
sudo rm /usr/local/var/run/yggdrasil/workers/device-worker.pid
```

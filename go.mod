module github.com/project-flotta/flotta-device-worker

go 1.16

require (
	git.sr.ht/~spc/go-log v0.0.0-20210611184941-ce2f05edb627
	github.com/Azure/azure-sdk-for-go v65.0.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.27 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.20 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/apenella/go-ansible v1.1.5
	github.com/aws/aws-sdk-go v1.44.37
	github.com/chzyer/readline v1.5.0 // indirect
	github.com/containers/podman/v4 v4.0.3
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/digitalocean/godo v1.80.0 // indirect
	github.com/docker/docker v20.10.16+incompatible // indirect
	github.com/edsrzf/mmap-go v1.1.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v0.6.7 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-openapi/runtime v0.23.1 // indirect
	github.com/go-openapi/strfmt v0.21.2
	github.com/go-openapi/swag v0.21.1
	github.com/godbus/dbus/v5 v5.1.0
	github.com/golang/mock v1.6.0
	github.com/golang/snappy v0.0.4
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0
	github.com/gophercloud/gophercloud v0.24.0 // indirect
	github.com/hashicorp/consul/api v1.12.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hetznercloud/hcloud-go v1.33.2 // indirect
	github.com/jaypipes/ghw v0.7.0
	github.com/linode/linodego v1.5.0 // indirect
	github.com/miekg/dns v1.1.49 // indirect
	github.com/onsi/ginkgo/v2 v2.1.4
	github.com/onsi/gomega v1.19.0
	github.com/opencontainers/runc v1.1.0
	github.com/openshift/assisted-installer-agent v1.0.10-0.20211027185717-53b0eacfa147
	github.com/pelletier/go-toml v1.9.4
	github.com/pkg/errors v0.9.1
	github.com/project-flotta/flotta-operator v0.1.1-0.20220707161136-6f40ced49d0f
	github.com/prometheus/client_golang v1.12.2
	github.com/prometheus/common v0.34.0
	github.com/prometheus/prometheus v1.8.2-0.20211217191541-41f1a8125e66
	github.com/redhatinsights/yggdrasil v0.0.0-20220323125707-cdde836c519a
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.9 // indirect
	github.com/seqsense/s3sync v1.8.3-0.20220617225540-7524fc039ded
	github.com/stretchr/testify v1.7.1
	github.com/vishvananda/netlink v1.1.1-0.20220115184804-dd687eb2f2d4
	golang.org/x/net v0.0.0-20220520000938-2e3eb7b945c2 // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a
	golang.org/x/time v0.0.0-20220224211638-0e9765cccd65 // indirect
	golang.org/x/xerrors v0.0.0-20220411194840-2f41105eb62f // indirect
	google.golang.org/genproto v0.0.0-20220524023933-508584e28198 // indirect
	google.golang.org/grpc v1.46.2
	gopkg.in/yaml.v3 v3.0.0 // indirect
	k8s.io/api v0.24.0
	k8s.io/apimachinery v0.24.0
	sigs.k8s.io/yaml v1.3.0
)

replace (
	github.com/containernetworking/cni => github.com/containernetworking/cni v0.8.1
	github.com/containers/buildah => github.com/containers/buildah v1.23.1
	github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.3-0.20220114050600-8b9d41f48198
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.4
	k8s.io/client-go => k8s.io/client-go v0.21.0
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20211115234752-e816edb12b65
	sigs.k8s.io/cluster-api-provider-aws => github.com/openshift/cluster-api-provider-aws v0.2.1-0.20201022175424-d30c7a274820
	sigs.k8s.io/cluster-api-provider-azure => github.com/openshift/cluster-api-provider-azure v0.1.0-alpha.3.0.20201016155852-4090a6970205
)

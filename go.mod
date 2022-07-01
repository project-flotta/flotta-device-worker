module github.com/project-flotta/flotta-device-worker

go 1.16

require (
	git.sr.ht/~spc/go-log v0.0.0-20210611184941-ce2f05edb627
	github.com/apenella/go-ansible v1.1.5
	github.com/aws/aws-sdk-go v1.44.19
	github.com/containers/podman/v4 v4.0.3
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/go-openapi/strfmt v0.21.1
	github.com/go-openapi/swag v0.19.15
	github.com/godbus/dbus/v5 v5.1.0
	github.com/golang/mock v1.6.0
	github.com/golang/snappy v0.0.4
	github.com/google/uuid v1.3.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jaypipes/ghw v0.7.0
	github.com/onsi/ginkgo/v2 v2.1.4
	github.com/onsi/gomega v1.19.0
	github.com/opencontainers/runc v1.1.0
	github.com/openshift/assisted-installer-agent v1.0.10-0.20211027185717-53b0eacfa147
	github.com/pelletier/go-toml v1.9.4
	github.com/pkg/errors v0.9.1
	github.com/project-flotta/flotta-operator v0.1.1-0.20220609063144-fb91b9b4f811
	github.com/prometheus/common v0.32.1
	github.com/prometheus/prometheus v1.8.2-0.20211217191541-41f1a8125e66
	github.com/redhatinsights/yggdrasil v0.0.0-20220323125707-cdde836c519a
	github.com/seqsense/s3sync v1.8.2
	github.com/stretchr/testify v1.7.1
	github.com/vishvananda/netlink v1.1.1-0.20220115184804-dd687eb2f2d4
	golang.org/x/sys v0.0.0-20220422013727-9388b58f7150
	google.golang.org/grpc v1.44.0
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	sigs.k8s.io/yaml v1.3.0
)

replace (
	github.com/containernetworking/cni => github.com/containernetworking/cni v0.8.1
	github.com/containers/buildah => github.com/containers/buildah v1.23.1
	github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.3-0.20220114050600-8b9d41f48198
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.4
	k8s.io/client-go => k8s.io/client-go v0.21.0
	sigs.k8s.io/cluster-api-provider-aws => github.com/openshift/cluster-api-provider-aws v0.2.1-0.20201022175424-d30c7a274820
	sigs.k8s.io/cluster-api-provider-azure => github.com/openshift/cluster-api-provider-azure v0.1.0-alpha.3.0.20201016155852-4090a6970205
)

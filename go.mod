module github.com/project-flotta/flotta-device-worker

go 1.16

require (
	git.sr.ht/~spc/go-log v0.0.0-20210611184941-ce2f05edb627
	github.com/apenella/go-ansible v1.1.5
	github.com/aws/aws-sdk-go v1.42.16
	github.com/blang/semver v3.5.1+incompatible
	github.com/containers/podman/v3 v3.4.3-0.20211216144417-90fb2cff071a
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/go-openapi/strfmt v0.21.1
	github.com/golang/mock v1.6.0
	github.com/golang/snappy v0.0.4
	github.com/google/uuid v1.3.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jaypipes/ghw v0.7.0
	github.com/onsi/ginkgo/v2 v2.1.3
	github.com/onsi/gomega v1.18.1
	github.com/openshift/assisted-installer-agent v1.0.10-0.20211027185717-53b0eacfa147
	github.com/pelletier/go-toml v1.9.4
	github.com/pkg/errors v0.9.1
	github.com/project-flotta/flotta-operator v0.1.0
	github.com/prometheus/common v0.32.1
	github.com/prometheus/prometheus v1.8.2-0.20211217191541-41f1a8125e66
	github.com/redhatinsights/yggdrasil v0.0.0-20220323125707-cdde836c519a
	github.com/seqsense/s3sync v1.8.0
	github.com/stretchr/testify v1.7.0
	github.com/vishvananda/netlink v1.1.1-0.20201029203352-d40f9887b852
	golang.org/x/sys v0.0.0-20220207234003-57398862261d
	google.golang.org/grpc v1.40.0
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	sigs.k8s.io/yaml v1.3.0
)

replace (
	github.com/containernetworking/cni => github.com/containernetworking/cni v0.8.1
	github.com/containers/buildah => github.com/containers/buildah v1.23.1
	github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2-0.20210819154149-5ad6f50d6283
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.4
	k8s.io/client-go => k8s.io/client-go v0.21.0
	sigs.k8s.io/cluster-api-provider-aws => github.com/openshift/cluster-api-provider-aws v0.2.1-0.20201022175424-d30c7a274820
	sigs.k8s.io/cluster-api-provider-azure => github.com/openshift/cluster-api-provider-azure v0.1.0-alpha.3.0.20201016155852-4090a6970205
)

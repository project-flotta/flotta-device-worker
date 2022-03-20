module github.com/project-flotta/flotta-device-worker

go 1.16

require (
	git.sr.ht/~spc/go-log v0.0.0-20210611184941-ce2f05edb627
	github.com/aws/aws-sdk-go v1.42.16
	github.com/containernetworking/plugins v1.0.1 // indirect
	github.com/containers/podman/v3 v3.4.3-0.20211216144417-90fb2cff071a
	github.com/containers/storage v1.37.1-0.20211014130921-5c5bf639ed01 // indirect
	github.com/go-openapi/strfmt v0.21.1
	github.com/golang/mock v1.6.0
	github.com/golang/snappy v0.0.4
	github.com/google/uuid v1.3.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.16.0
	github.com/opencontainers/runtime-tools v0.9.1-0.20211020193359-09d837bf40a7 // indirect
	github.com/openshift/assisted-installer-agent v1.0.10-0.20211027185717-53b0eacfa147
	github.com/pkg/errors v0.9.1
	github.com/project-flotta/flotta-operator v0.0.2-0.20220314112640-9186994c5b6e
	github.com/prometheus/common v0.32.1
	github.com/prometheus/prometheus v1.8.2-0.20211217191541-41f1a8125e66
	github.com/redhatinsights/yggdrasil v0.0.0-20220309180936-d91759a7f332
	github.com/seqsense/s3sync v1.8.0
	google.golang.org/grpc v1.40.0
	k8s.io/api v0.22.4
	k8s.io/apimachinery v0.22.4
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/containernetworking/cni => github.com/containernetworking/cni v0.8.1
	github.com/containers/buildah => github.com/containers/buildah v1.23.1
	github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2-0.20210819154149-5ad6f50d6283
	k8s.io/client-go => k8s.io/client-go v0.21.0
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.2.0
	sigs.k8s.io/cluster-api-provider-aws => github.com/openshift/cluster-api-provider-aws v0.2.1-0.20201022175424-d30c7a274820
	sigs.k8s.io/cluster-api-provider-azure => github.com/openshift/cluster-api-provider-azure v0.1.0-alpha.3.0.20201016155852-4090a6970205
)

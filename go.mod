module github.com/jakub-dzon/k4e-device-worker

go 1.16

require (
	git.sr.ht/~spc/go-log v0.0.0-20210611184941-ce2f05edb627
	github.com/aws/aws-sdk-go v1.35.28
	github.com/containers/podman/v3 v3.2.0-rc1.0.20211108082956-865653b661dd
	github.com/go-openapi/strfmt v0.20.0
	github.com/golang/mock v1.6.0
	github.com/google/uuid v1.3.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jakub-dzon/k4e-operator v0.0.0-20211129124435-655221702db6
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.16.0
	github.com/openshift/assisted-installer-agent v1.0.10-0.20211027185717-53b0eacfa147
	github.com/redhatinsights/yggdrasil v0.0.0-20210630184700-1d2d42276b6a
	github.com/seqsense/s3sync v1.8.0
	google.golang.org/grpc v1.40.0
	k8s.io/api v0.22.3
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/containers/buildah => github.com/containers/buildah v1.23.1
	github.com/containers/common => github.com/containers/common v0.46.1-0.20211026130826-7abfd453c86f
	github.com/redhatinsights/yggdrasil => github.com/jakub-dzon/yggdrasil v0.0.0-20211012071055-27d969343f4e
	k8s.io/client-go => k8s.io/client-go v0.21.0
	sigs.k8s.io/cluster-api-provider-aws => github.com/openshift/cluster-api-provider-aws v0.2.1-0.20201022175424-d30c7a274820
	sigs.k8s.io/cluster-api-provider-azure => github.com/openshift/cluster-api-provider-azure v0.1.0-alpha.3.0.20201016155852-4090a6970205
)

module github.com/jakub-dzon/k4e-device-worker

go 1.16

require (
	git.sr.ht/~spc/go-log v0.0.0-20210611184941-ce2f05edb627
	github.com/aws/aws-sdk-go v1.35.28
	github.com/containers/buildah v1.21.0 // indirect
	github.com/containers/common v0.38.11 // indirect
	github.com/containers/podman/v2 v2.2.1
	github.com/containers/psgo v1.5.2 // indirect
	github.com/containers/storage v1.31.3 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/cri-o/ocicni v0.2.1-0.20210621164014-d0acc7862283 // indirect
	github.com/go-openapi/strfmt v0.20.0
	github.com/golang/mock v1.6.0
	github.com/google/uuid v1.2.0
	github.com/jakub-dzon/k4e-operator v0.0.0-20211007063427-678c7225ae78
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.16.0
	github.com/openshift/assisted-installer-agent v1.0.10-0.20210609144611-8a8c4c4c4969
	github.com/redhatinsights/yggdrasil v0.0.0-20210630184700-1d2d42276b6a
	github.com/seqsense/s3sync v1.8.0
	google.golang.org/grpc v1.39.0
	k8s.io/api v0.21.0
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/containers/buildah => github.com/containers/buildah v1.18.0
	github.com/containers/common => github.com/containers/common v0.29.0
	k8s.io/client-go => k8s.io/client-go v0.21.0
	sigs.k8s.io/cluster-api-provider-aws => github.com/openshift/cluster-api-provider-aws v0.2.1-0.20201022175424-d30c7a274820
	sigs.k8s.io/cluster-api-provider-azure => github.com/openshift/cluster-api-provider-azure v0.1.0-alpha.3.0.20201016155852-4090a6970205

)

replace github.com/redhatinsights/yggdrasil => github.com/jakub-dzon/yggdrasil v0.0.0-20211012071055-27d969343f4e

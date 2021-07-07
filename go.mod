module github.com/jakub-dzon/k4e-device-worker

go 1.14

require (
	git.sr.ht/~spc/go-log v0.0.0-20210611184941-ce2f05edb627
	github.com/containers/buildah v1.21.0 // indirect
	github.com/containers/common v0.38.11 // indirect
	github.com/containers/podman/v2 v2.2.1
	github.com/containers/psgo v1.5.2 // indirect
	github.com/containers/storage v1.31.3 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/cri-o/ocicni v0.2.1-0.20210621164014-d0acc7862283 // indirect
	github.com/go-openapi/strfmt v0.19.5
	github.com/google/uuid v1.2.0
	github.com/jakub-dzon/k4e-operator v0.0.0-20210707090443-1feee007c7ff
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/redhatinsights/yggdrasil v0.0.0-20210630184700-1d2d42276b6a
	google.golang.org/grpc v1.39.0
	k8s.io/api v0.21.0
	sigs.k8s.io/yaml v1.2.0

)

replace (
	github.com/containers/common => github.com/containers/common v0.29.0
	github.com/containers/buildah => github.com/containers/buildah v1.18.0
)

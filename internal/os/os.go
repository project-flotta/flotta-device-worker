package os

const (
	UnknownOsImageId = "unknown"
)

type OS struct {
}

func (o *OS) GetOsImageId() string {
	return UnknownOsImageId
}

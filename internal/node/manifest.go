package node

import (
	sharedconfig "github.com/kcodes0/decent/internal/config"
	"github.com/kcodes0/decent/internal/protocol"
)

func LoadManifest(path string) (protocol.Manifest, error) {
	manifest, err := sharedconfig.ReadManifestPath(path)
	if err != nil {
		return protocol.Manifest{}, err
	}
	return *manifest, nil
}

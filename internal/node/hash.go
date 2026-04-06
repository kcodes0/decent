package node

import (
	"fmt"
	"net"

	sharedconfig "github.com/kcodes0/decent/internal/config"
	"github.com/kcodes0/decent/internal/content"
)

func HashTree(root string) (string, error) {
	return content.HashTree(root, sharedconfig.ManifestFileName)
}

func joinURL(host string, port int) string {
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
}

package listeners

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/go-connections/sockets"
)

// Init creates new listeners for the server.
func Init(proto, addr, socketGroup string, tlsConfig *tls.Config) (ls []net.Listener, err error) {
	switch proto {
	case "tcp":
		l, err := sockets.NewTCPSocket(addr, tlsConfig)
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)
	case "unix":
		gid, err := lookupGID(socketGroup)
		if err != nil {
			if socketGroup != defaultSocketGroup {
				return nil, err
			}
			logrus.Warnf("could not change group %s to %s: %v", addr, defaultSocketGroup, err)
			gid = os.Getgid()
		}
		l, err := sockets.NewUnixSocket(addr, gid)
		if err != nil {
			return nil, fmt.Errorf("can't create unix socket %s: %v", addr, err)
		}
		ls = append(ls, l)
	default:
		return nil, fmt.Errorf("Invalid protocol format: %q", proto)
	}

	return
}

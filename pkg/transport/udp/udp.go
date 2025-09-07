package udp

import "github.com/amirimatin/go-cluster/pkg/transport"

// impl is a trivial Transport that only carries an address string.
type impl struct {
    addr string
}

func (i *impl) Addr() string { return i.addr }

// New constructs a stub UDP transport that satisfies the transport.Transport
// interface. It does not open sockets in this phase.
func New(addr string) (transport.Transport, error) {
    return &impl{addr: addr}, nil
}


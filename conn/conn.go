package conn

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
)

// Conn is an abstract of a unix datagram connection so that a test mock can be probided
type Conn interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
}

// ListenConn is the "server-like" side of a unix datagram connection that needs to know the
// sender address of a datagram it received, in order to send a response.
type ListenConn interface {
	Conn
	ReadFrom([]byte) (int, net.Addr, error)
	Get(net.Addr) (Conn, error)
	Dial() (Conn, error)
}

// unixListenConn is the actual unix datagram sockets implementation
type unixListenConn struct {
	endpoint string
	iface    string
	*net.UnixConn
}

// NewUnixListen creates a new ListeConn -- this is used for tests.
func NewUnixListen(endpoint, iface string) (ListenConn, error) {
	wpaEndpoint := path.Join(endpoint, iface)
	c, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: wpaEndpoint})
	if err != nil {
		return nil, err
	}
	return &unixListenConn{endpoint, iface, c}, nil
}

// NewUnixConn creates a unix datagram socket sender with a localaddress attached the
// the receiver can use for sending responses.
func NewUnixConn(endpoint, iface string) (Conn, error) {
	f, err := ioutil.TempFile("", fmt.Sprintf("wpactrl-%s", iface))
	if err != nil {
		return nil, err
	}
	os.Remove(f.Name())
	f.Close()

	recvEndpoint := f.Name()
	os.Remove(recvEndpoint)

	wpaEndpoint := path.Join(endpoint, iface)

	return net.DialUnix("unixgram",
		&net.UnixAddr{Name: recvEndpoint},
		&net.UnixAddr{Name: wpaEndpoint},
	)
}

// Get will return a connection to respond on
func (uc *unixListenConn) Get(addr net.Addr) (Conn, error) {
	uaddr, ok := addr.(*net.UnixAddr)
	if !ok {
		return nil, errors.New("need *net.UnixAddr")
	}
	return net.DialUnix("unixgram", nil, uaddr)
}

// Dial is used for testing, this returns a connection to the listening connection
func (uc *unixListenConn) Dial() (Conn, error) {
	return NewUnixConn(uc.endpoint, uc.iface)
}

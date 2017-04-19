package protocol

import (
	"net"
)

type CommProtocol struct {

}

func (this *CommProtocol) NewCodec(conn *net.TCPConn) Conn {
	return NewCommCodec(conn)
}

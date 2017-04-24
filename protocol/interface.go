package protocol

import (
	"net"
)

type Message interface {
	Serialize() ([]byte, error)
}

type Conn interface {
	Read() (msg Message, e error)
	Write(msg Message) (n int, err error)
	WriteBinary(msg []byte) (n int, err error)
	Close() error
	DoHeartBeat() error
}

type Protocol interface {
	NewCodec(conn *net.TCPConn) Conn
}
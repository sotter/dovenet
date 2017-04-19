package base

import (
	"errors"
	"fmt"
	"github.com/sotter/dovenet/protocol"
)

var (
	ErrorParameter error = errors.New("Parameter error")
	ErrorNilKey error = errors.New("Nil key")
	ErrorNilValue error = errors.New("Nil value")
	ErrorWouldBlock error = errors.New("Would block")
	ErrorNotHashable error = errors.New("Not hashable")
	ErrorNilData error = errors.New("Nil data")
	ErrorIllegalData error = errors.New("More than 8M data")
	ErrorNotImplemented error = errors.New("Not implemented")
	ErrorConnClosed error = errors.New("Connection closed")
)

const (
	WORKERS = 20
	MAX_CONNECTIONS = 1000
)

func Undefined(msgType int32) error {
	return ErrorUndefined{
		msgType: msgType,
	}
}

type ErrorUndefined struct {
	msgType int32
}

func (eu ErrorUndefined) Error() string {
	return fmt.Sprintf("Undefined mls.Message %d", eu.msgType)
}

//底层通知上的网络处理
type NetworkCallBack interface {
	OnMessageData(conn *TcpConnection, msg protocol.Message) error
	OnConnection(conn *TcpConnection)
	OnDisConnection(conn *TcpConnection)
}

//主动发的操作
type NetIOAction interface {
	SendData(name string, msg protocol.Message)
	BroadCast(name string, msg protocol.Message)
	Close(connection *TcpConnection)
}


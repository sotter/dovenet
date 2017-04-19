package base

import (
	"net"
	"sync"
	"errors"
	"github.com/sotter/dovenet/protocol"
	log "github.com/sotter/dovenet/log"
)

type TCPServer struct {
	isRunning bool
	address   string
	listener  net.Listener
	once      sync.Once       		// Promise Do Once
	Manager   *Manager        		// TcpSession的管理
	Protocol  protocol.Protocol  	// Protocol -> Make Codec
	NetworkCB NetworkCallBack 		// TcpConnection callBack
	//CryptInfo mls.Info        		// For TLS Config
}

func NewTCPServer(address string, p protocol.Protocol) (*TCPServer, error) {
	log.Println("New Server listen at : ", address)
	l, err := net.Listen("tcp4", address)
	if err != nil {
		return nil, err
	}

	return &TCPServer{
		isRunning: true,
		listener : l,
		Manager: NewManager(),
		once: sync.Once{},
		Protocol: p,
		address: address,
	}, nil
}

func (this *TCPServer) Accept() (net.Conn, error) {
	conn, err := this.listener.Accept()
	if err != nil {
		log.Println("TcpServer -> Accept : ", err.Error())
		return nil, err
	} else {
		return conn, err
	}
}

//根据ConnId发送数据
func (this *TCPServer) SendData(connId uint64, msg protocol.Message) error {
	session := this.Manager.GetSession(connId)
	if session == nil {
		return errors.New("can not get session!")
	}

	return session.Write(msg)
}

func (this *TCPServer) Stop() {
	this.listener.Close()
	this.Manager.Dispose()
}

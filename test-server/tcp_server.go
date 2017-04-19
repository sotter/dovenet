package main

import _ "net/http/pprof"

import (
	"github.com/sotter/dovenet/base"
	"github.com/sotter/dovenet/protocol"
	log "github.com/sotter/dovenet/log"
	"time"
	"net"
	"runtime"
	"net/http"
)

type TestServer struct {
	TcpServer     *base.TCPServer
	ServerNumber  uint16
}

type Session struct {
	TcpConn *base.TcpConnection
	//其他扩展数据
}

//直接使用Session的回调，可以减少一次从TcpConnection层到Session层的查找
func (this *Session) OnMessageData(conn *base.TcpConnection, msg protocol.Message) error{
	recv_msg := msg.(*protocol.CommMsg)
	log.Print("Recv msg : ", string(recv_msg.Body))
	conn.Write(protocol.NewCommMsg(0x1122, []byte("Hello, I am server")))

	return nil
}

func (this *Session)OnConnection(conn *base.TcpConnection) {
	log.Print("New connection from ", conn.Address)
}

func (this *Session)OnDisConnection(conn *base.TcpConnection) {
	log.Print("DisConnection : ", conn.Address)
}

func (this *TestServer)SendData(connId uint64, msg protocol.Message) {
	tcpConn := this.TcpServer.Manager.GetSession(connId)
	tcpConn.Write(msg)
}

func (this *TestServer)GetServerNumber() uint16 {
	//log.Info("Get Server Number : ", this.ServerNumber)
	return this.ServerNumber
}

func (this *TestServer)Loop() {
	//单独绑定一个线程
	defer base.RecoverPrint()
	runtime.LockOSThread()

	for {
		tcpConn, err := this.TcpServer.Accept()
		if err != nil {
			log.Print("Accept ", err.Error())
			continue
		}

		//如果4分钟没有任何数据，断开连接
		//tcpConn.SetDeadline(4 * time.Minute)
		session := Session{
		}

		tcpConn.SetReadDeadline(time.Now().Add(time.Minute*4))

		tcpConnection:= base.NewServerConn(
			base.GetNetId(),
			this.TcpServer.Protocol.NewCodec(tcpConn.(* net.TCPConn)),
			&session,
			this.TcpServer.Manager)

		tcpConnection.Address = tcpConn.RemoteAddr().String()
		tcpConnection.NetworkCB = &session
		session.TcpConn = tcpConnection
		tcpConnection.ConnManager  = this.TcpServer.Manager

		tcpConnection.Start()
	}
}

func main() {
	go func() {
		log.Print(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	tcp_server, _ := base.NewTCPServer(":8000", &protocol.CommProtocol{})
	test_server := TestServer {
		ServerNumber : 1234,
		TcpServer    : tcp_server,
	}

	test_server.Loop()
}

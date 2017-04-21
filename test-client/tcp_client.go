package main

import (
	"net"
	"time"
	log "github.com/sotter/dovenet/log"
	"github.com/sotter/dovenet/base"
	"github.com/sotter/dovenet/protocol"
)

type ServiceClient struct {
	ChanSize  uint32
	Transport *base.TransPortClient
}

var (
	G_SERVICE_CLIENT = NewServiceClient(10 * 1024)
)

//给msg handler使用
func SendData(name string, msg protocol.Message) error {
	return G_SERVICE_CLIENT.Transport.SendData(name, msg)
}

func NewServiceClient(size uint32) *ServiceClient {
	return &ServiceClient{
		ChanSize:size,
		Transport : base.NewTransPortClient(),
	}
}

func (this *ServiceClient) RegisterClient(name string, address string) {
	//1.  建立网络连接net.TcpConn
	dest, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		log.Print(err.Error())
		this.Reconnect(name, address)
		return
	}

	//2. 生成TcpConnection
	protocol := protocol.CommProtocol{}
	tcpConn := base.NewClientConn(base.GetNetId(),  protocol.NewCodec(dest.(*net.TCPConn)), this.ChanSize, this)
	tcpConn.Name = name
	tcpConn.Address = address
	tcpConn.WorkNum = 1     //每一种类型的建立一个连接

	//3. 放入到TransPortClient， 并启动
	if err := this.Transport.RegisterConnServer(tcpConn); err == nil {
		tcpConn.Start()
	}
}

//提供给动态配置对外回调
func (this *ServiceClient)AddClient(name string, address string) {
	//5s后发起连接，防止IP连接不上时发生阻塞
	log.Print("Config Modify -> Add Client ", name, " ", address)
	go this.RegisterClient(name, address)
}

//提供给动态配置对外回调
func (this *ServiceClient)RemoveClient(name string, address string) {
	log.Print("Config Modify -> Remove Client ", name, " ", address)
	this.Transport.RemoveByAddress(name, address)
}

//业务处理函数
func (this *ServiceClient) OnMessageData(conn *base.TcpConnection, msg protocol.Message) error {
	recv_msg := msg.(*protocol.CommMsg)
	log.Print("recv from ", conn.Address, " ", string(recv_msg.Body))
	return nil
}

func (this *ServiceClient) OnConnection(conn *base.TcpConnection) {
	log.Print("On Connection from ", conn.Address)
	//this.DoHeartBeatLoop(conn.Name, conn.ConnID)
	conn.Write(protocol.NewCommMsg(0x0022, []byte("Hello, I am Client")))
}

func (this *ServiceClient) OnDisConnection(conn *base.TcpConnection) {
	log.Print("On Disconnection from ", conn.Address)

	if conn.Reconnect {
		this.Reconnect(conn.Name, conn.Address)
	}
}

func (this *ServiceClient) Reconnect(name string, address string) {
	time.AfterFunc(5*time.Second, func() {
		this.RegisterClient(name, address)
	})
}

func (this *ServiceClient) DoHeartBeatLoop(name string, connId uint64) {
	conn := this.Transport.GetTcpConnection(name, connId)
	if conn != nil {
		//DoHeartBeat(conn)
		//只有找到才有下一次的心跳计时，如果没有找到说明在这个周期中conn已经关闭了
		time.AfterFunc(30 * time.Second, func() {
			this.DoHeartBeatLoop(name, connId)
		})
	}
}

func main() {
	test_client := NewServiceClient(1024)
	test_client.RegisterClient("ctl_client", "127.0.0.1:8000")

	select {

	}
}
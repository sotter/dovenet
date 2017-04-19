package  base

import (
	"errors"
	"sync"
	"github.com/sotter/dovenet/protocol"
	log "github.com/sotter/dovenet/log"
)

const (
	STATUS_CLIENT = iota
	CLIENT_TYPE_NUM
)

func GetClientTypeByName(name string) (int32, error) {
	switch  {
	case name == "status_client":
		return STATUS_CLIENT, nil
	default:
		return  CLIENT_TYPE_NUM, errors.New("can not find Client name: " + name)
	}
}

func CheckNameValid(name string) bool {
	switch {
	case name == "status_client":
		return true
	default:
		return false
	}
}

//针对某一个类型Server的连接地址，转发的时候方便
type ServerConnGroup struct {
	stop        chan bool
	name        string
	Manager     *Manager    //派发方式在Manage中实现
}

//TCP Client管理的总入口，所有的客户端可以用一个全局的TransPortClient来管理
type TransPortClient struct {
	stop        chan bool
	connGroups  [CLIENT_TYPE_NUM]ServerConnGroup  // 连接池管理， key：连接的Name，eg: lb_client
	once        *sync.Once
	lock        sync.RWMutex
}

func NewTransPortClient() *TransPortClient {
	tps := &TransPortClient{
		stop : make(chan bool, 1),
		once : &sync.Once{},
	}

	for i := 0; i < CLIENT_TYPE_NUM; i++ {
		tps.connGroups[i].Manager = NewManager()
	}

	return tps
}

//在外部创建TcpConnection
func (this *TransPortClient)RegisterConnServer(tcpConn *TcpConnection) (err error){
	defer RecoverPrint()
	if client_type, err := GetClientTypeByName(tcpConn.Name); err == nil {
		//log.Debug("======== this *TransPortClient)RegisterConnServer client type ", client_type)
		this.connGroups[client_type].Manager.PutSession(tcpConn)
		tcpConn.ConnManager = this.connGroups[client_type].Manager
	}

	return err
}

//根据连接客户端的Name和连接id获取到TcpConnection
func (this *TransPortClient)GetTcpConnection(name string, connid uint64)  *TcpConnection {
	client_type, err := GetClientTypeByName(name)
	if err != nil {
		log.Println("GetTcpConnection :", err.Error())
		return nil
	}

	return this.connGroups[client_type].Manager.GetSession(connid)
}

func (this *TransPortClient)RemoveByAddress(name string, address string) {
	client_type, err := GetClientTypeByName(name)
	if err != nil {
		log.Println("RemoveByAddress :", err.Error())
		return
	}

	conns := this.connGroups[client_type].Manager.GetSessionByAddress(address)

	for _, conn := range conns {
		conn.Reconnect = false
		conn.Close()
	}
}

//一层一层的往下关闭
func (this *TransPortClient)Stop() {
	for i := 0; i < CLIENT_TYPE_NUM; i++ {
		this.connGroups[i].Manager.Dispose()
	}
}

func (this *TransPortClient)SendData(name string, msg protocol.Message) error {
	defer RecoverPrint()

	index, err := GetClientTypeByName(name)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	if index >= CLIENT_TYPE_NUM {
		return errors.New("can not find client name!!")
	}
	tcpConn := this.connGroups[index].Manager.GetRotationSession()
	if tcpConn == nil {
		return errors.New("Can not get connection!!")
	}

	return tcpConn.Write(msg)
}

func (this *TransPortClient)BroadCast(name string, msg protocol.Message) error {
	defer RecoverPrint()
	index, err := GetClientTypeByName(name)
	if err != nil {
		return err
	}

	if index >= CLIENT_TYPE_NUM {
		return errors.New("can not find client name!!")
	}

	this.connGroups[index].Manager.BroadcastRun(func(conn *TcpConnection){
		conn.Write(msg)
	})

	return nil
}

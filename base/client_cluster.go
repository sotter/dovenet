package  base

import (
	"errors"
	"sync"
	"github.com/sotter/dovenet/protocol"
	log "github.com/sotter/dovenet/log"
)

//针对某一个类型Server的连接地址，转发的时候方便
type ServerConnGroup struct {
	stop        chan bool
	name        string
	Manager     *Manager    //派发方式在Manage中实现
}

//TCP Client管理的总入口，所有的客户端可以用一个全局的TransPortClient来管理
type TransPortClient struct {
	stop        chan bool
	connGroups  []*ServerConnGroup  // 连接池管理， key：连接的Name，eg: lb_client
	connsIndex	map[string]int
	once        *sync.Once
	lock        sync.RWMutex
}

func NewServerConnGroup(name string) *ServerConnGroup {
	return &ServerConnGroup {
		stop : make(chan bool),
		name : name,
		Manager: NewManager(),
	}
}

func NewTransPortClient() *TransPortClient {
	tps := &TransPortClient{
		stop : make(chan bool, 1),
		connsIndex : make(map[string]int),
		once : &sync.Once{},
	}

	return tps
}

//在外部创建TcpConnection
func (this *TransPortClient)RegisterConnServer(tcpConn *TcpConnection) (err error) {
	defer RecoverPrint()

	//如果没有这个Manager，那么注册下这个manager
	this.RegisterConnGroup(tcpConn.Name)

	this.lock.RLock()
	defer this.lock.RUnlock()
	index, exist:= this.connsIndex[tcpConn.Name]
	if exist {
		this.connGroups[index].Manager.PutSession(tcpConn)
		tcpConn.ConnManager = this.connGroups[index].Manager
	} else {
		log.Println("Server Name ", tcpConn.Name, " can not find")
		return errors.New("Can find Server Name")
	}

	return err
}


//根据连接客户端的Name和连接id获取到TcpConnection
func (this *TransPortClient)GetTcpConnection(name string, connid uint64)  *TcpConnection {
	this.lock.RLock()
	defer this.lock.RUnlock()

	index, exist := this.connsIndex[name]
	if exist == false {
		return nil
	}
	return this.connGroups[index].Manager.GetSession(connid)
}

//注册一个新的ClientName
func (this *TransPortClient)RegisterConnGroup(name string) {
	this.lock.Lock()
	defer this.lock.Unlock()
	_, exist := this.connsIndex[name]
	if exist {
		return
	}

	cg := NewServerConnGroup(name)
	this.connGroups = append(this.connGroups, cg)
	this.connsIndex[name] =  len(this.connGroups) - 1
}

func (this *TransPortClient)RemoveByAddress(name string, address string) {
	this.lock.Lock()
	index, exist := this.connsIndex[name]
	if exist == false {
		log.Println("RemoveByAddress :", name, " is not exist!")
		this.lock.Unlock()
		return
	} else {
		conns := this.connGroups[index].Manager.GetSessionByAddress(address)
		delete(this.connsIndex, name)
		this.lock.Unlock()

		for _, conn := range conns {
			conn.Reconnect = false
			conn.Close()
		}
	}
}

//一层一层的往下关闭
func (this *TransPortClient)Stop() {
	n := len(this.connGroups)
	for i := 0; i < n; i++ {
		this.connGroups[i].Manager.Dispose()
	}
}

func (this *TransPortClient)SendData(name string, msg protocol.Message) error {
	defer RecoverPrint()

	this.lock.RLock()
	var tcpConn *TcpConnection = nil
	index, exist := this.connsIndex[name]
	if exist {
		tcpConn = this.connGroups[index].Manager.GetRotationSession()
	}
	this.lock.RUnlock()

	if tcpConn != nil {
		return tcpConn.Write(msg)
	} else {
		return errors.New("No Tcpconnecion can use.")
	}
}

func (this *TransPortClient)BroadCast(name string, msg protocol.Message) error {
	defer RecoverPrint()

	this.lock.RLock()
	index, exist := this.connsIndex[name]
	if exist {
		this.connGroups[index].Manager.BroadcastRun(func(conn *TcpConnection){
			conn.Write(msg)
		})
	}
	this.lock.RUnlock()

	return nil
}

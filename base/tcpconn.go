package base

import (
	"sync"
	"time"
	"sync/atomic"
	"fmt"
	"github.com/sotter/dovenet/protocol"
	log "github.com/sotter/dovenet/log"
)

const (
	NTYPE  = 4
	NLEN   = 4
	MAXLEN = 1 << 23  // 8M
)

//socket state
const (
	CLOSED = iota
	CONNECTING
	ESTABLISHED
	LISTEN
	SOCK_STATE_NUM
)

//socket type
const (
	TCP_LISTEN = iota
	TCP_SVR_CONN
	TCP_CLIENT_CONN
	UDP_SOCK
	SOCK_TYPE_NUM
)

type TcpConnection struct {
	//连接ID，后面生成的规则要结合ServerNumber
	ConnID             uint64

	//此连接对应的业务连接的名字， 可以封装到Session那一层中
	Name               string

	//对于ListenSocket来说，是监听socket，对于其他socket是对端的地址
	Address            string
	ConnState          uint8
	ConnType           uint8

	//底层采用什么样的分包
	conn               protocol.Conn

	ConnManager        *Manager
	running            int32
	once               sync.Once
	finish             sync.WaitGroup

	HeartBeat          bool
	heartBeatInterval  time.Duration

	//异步数据发送队列
	messageSendChan    chan protocol.Message
	messageHandlerChan chan protocol.Message
	closeConnChan      chan struct{}

	//业务事件的回调
	NetworkCB          NetworkCallBack
	Reconnect          bool

	//时间戳, 用作RT计算
	TimeStamp          time.Time

	//Work协程的数目
	WorkNum            int

	// 扩展数据，可以放到Session层中
	ExtraData          interface{}
}

//创建Server派生的客户端
func NewServerConn(connid uint64, conn protocol.Conn, networkcb NetworkCallBack, m *Manager) *TcpConnection {
	serverConn := &TcpConnection{
		ConnID: connid,
		conn: conn,
		running: 1,
		ConnState: CLOSED,
		ConnType: TCP_SVR_CONN,

		HeartBeat : false,
		heartBeatInterval : 0,

		ConnManager:m,
		finish: sync.WaitGroup{},
		messageSendChan: make(chan protocol.Message, 128),
		messageHandlerChan: make(chan protocol.Message, 128),
		closeConnChan: make(chan struct{}),

		NetworkCB: networkcb,
		Reconnect : false,

		WorkNum : 1,
	}

	//添加去ga
	if serverConn.ConnManager != nil {
		serverConn.ConnManager.PutSession(serverConn)
	}

	return serverConn
}

//创建主动发起的客户端, 如果是客户端，首先要能连接得上，如果连接不上则添加不上；
func NewClientConn(connId uint64, conn protocol.Conn, chanSize uint32, networkcb NetworkCallBack) (*TcpConnection) {
	return &TcpConnection{
		ConnID: connId,

		running: 1,
		conn: conn,
		ConnState: CLOSED,
		ConnType: TCP_CLIENT_CONN,

		HeartBeat : true,
		heartBeatInterval : 30 * time.Second,

		finish: sync.WaitGroup{},
		messageSendChan: make(chan protocol.Message, chanSize),
		messageHandlerChan: make(chan protocol.Message, chanSize),
		closeConnChan: make(chan struct{}),
		NetworkCB: networkcb,
		WorkNum : 16,
		Reconnect : true,
	}
}

//记录当前TcpConnection的信息
func (this *TcpConnection)String() string {
	return fmt.Sprint(this.ConnID , ":",  this.Address)
}

func (this *TcpConnection)Start() bool {
	//!!!注意：这个地方finish.Add要在routine启动之前调用，如果在routine里面调用，
	// 可能会出现已经处于wait状态，但是Add还没有调用，此时会有panic
	this.finish.Add(2 + this.WorkNum)
	go this.readLoop()
	go this.writeLoop()
	for i := 0; i < this.WorkNum; i++ {
		go this.workLoop()
	}

	//OnConnectiong 放到所有协程启动之后，优点：如果OnConnection有业务不会阻塞运行；
	// 缺点:如果连接上来，立马关闭的业务，会有损耗； 是否有隐藏的坑，暂未发现； 2017-02-14
	this.NetworkCB.OnConnection(this)
	return true
}

func (this *TcpConnection)Stop() bool {
	return true
}

func (this *TcpConnection) DoHeartBeat() error {
	return this.conn.DoHeartBeat()
}

func (this *TcpConnection)Write(msg protocol.Message) (err error) {
	select {
	case this.messageSendChan <- msg:
		return nil
	default:
		//calc.Add("Lost Packet")
		log.Println("messageSendChan is full , Write Lost packet !!!")
		return nil
	}
}

//TODO : 目前先改成，如果已经发送不出去了，直接把调用者阻塞堵住, 以应对可靠性要求高和具有流控功能的业务
func (this *TcpConnection)WriteWouldBlock(msg protocol.Message) (err error) {
	select {
	case this.messageSendChan <- msg:
		return nil
	}
}

func (this *TcpConnection)WriteBinary(msg []byte) (n int, err error) {
	return this.conn.WriteBinary(msg)
}

//TODO: 与解包器结合起来；
func (this *TcpConnection) Read() (err error) {
	//底层已经自动解包，并且屏蔽了加密层
	msg, err := this.conn.Read()
	if err != nil {
		log.Println("ReadData fail address ", this.Address)
		return err
	}

	this.messageHandlerChan <- msg

	return nil
}

func (this *TcpConnection)readLoop() {
	defer RecoverPrint()

	//this.finish.Add(1)
	defer func () {
		this.finish.Done()
		this.Close()
	}()

	for atomic.LoadInt32(&this.running) == 1 {
		select {
		case <-this.closeConnChan:
			log.Println("readLoop -> To Close ", this.String())
			return
		default:
			err := this.Read()
			if err != nil {
				log.Println("ReadLoop -> To Close:", err.Error() , " ", this.String())
				return
			}
		}
	}
}

func (this *TcpConnection)writeLoop() {
	defer RecoverPrint()

	//this.finish.Add(1)
	defer func () {
		this.finish.Done()
		this.Close()
	}()

	for atomic.LoadInt32(&this.running) == 1  {
		select {
		case <-this.closeConnChan:
			log.Println("writeLoop -> To Close", this.String())
			return

		case msg := <-this.messageSendChan:
			if msg != nil {
				if _, err := this.conn.Write(msg); err != nil {
					log.Println("Error writing data ", err.Error(), " ", this.String())
					return
				}
			}
		}
	}
}

//将解包好的数据拿出来，进行OnMessageData处理
func (this *TcpConnection)workLoop() {
	defer RecoverPrint()

	//this.finish.Add(1)
	defer func () {
		this.finish.Done()
		this.Close()
	}()

	for atomic.LoadInt32(&this.running) == 1  {
		select {
		case <-this.closeConnChan:
			log.Println("workLoop -> To Close ", this.String())
			return

		case msg := <-this.messageHandlerChan:
			if msg != nil {
				if err := this.NetworkCB.OnMessageData(this, msg); err != nil {
					//TODO: 是否需要关闭， 对于Gateway跟后面业务服务器的连接是不能关闭的；
				}
			}
		}
	}
}

func (this *TcpConnection)Close() {
	//下面有this.conn.Close的调用，所以要加上两层防护；
	this.once.Do(func() {
		//把running设置为0， 同时保证下面的代码只会被执行一次
		if atomic.CompareAndSwapInt32(&this.running, 1, 0) {
			//通知给上一层关闭
			this.NetworkCB.OnDisConnection(this)

			//对于Server端来说，从Manager的管理中删除掉
			if this.ConnManager != nil {
				this.ConnManager.delSession(this)
			}

			//关闭掉channel，所有的work都要监测closeConnChan
			close(this.messageSendChan)
			close(this.messageHandlerChan)
			close(this.closeConnChan)

			this.conn.Close()
			this.ConnState = CLOSED

			this.finish.Wait()
		}
	})
}

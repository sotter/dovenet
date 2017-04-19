package base
//主要用于由Server派生的TcpConnection的管理

import (
	"sync"
	"math/rand"
	"time"
)

//把一个大的Map，分成好多小的map管理，这样就减少了锁的粒度
const sessionMapNum = 8

type Manager struct {
	sessionMaps [sessionMapNum]sessionMap
	disposeFlag bool
	disposeOnce sync.Once
	disposeWait sync.WaitGroup

	current     uint64
}

type sessionMap struct {
	sync.RWMutex
	sessions map[uint64]*TcpConnection
}

func (this *sessionMap) RandomSelectFromMap() *TcpConnection {
	var array_conns []*TcpConnection
	for _, session := range this.sessions {
		array_conns = append(array_conns, session)
	}

	if len(array_conns) == 0 {
		return nil
	}

	index := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(len(array_conns))

	return array_conns[index]
}

func NewManager() *Manager {
	manager := &Manager{}
	for i := 0; i < len(manager.sessionMaps); i++ {
		manager.sessionMaps[i].sessions = make(map[uint64]*TcpConnection)
	}

	return manager
}

func (this *Manager) Dispose() {
	this.disposeOnce.Do(func() {
		this.disposeFlag = true
		for i := 0; i < sessionMapNum; i++ {
			smap := &this.sessionMaps[i]
			smap.Lock()
			for _, session := range smap.sessions {
				session.Close()
			}
			smap.Unlock()
		}
		this.disposeWait.Wait()
	})
}

//func (manager *Manager) NewSession(codec Codec, sendChanSize int) *TcpConnection {
//	session := newSession(manager, codec, sendChanSize)
//	manager.PutSession(session)
//	return session
//}

func (this *Manager) GetSession(sessionID uint64) *TcpConnection {
	smap := &this.sessionMaps[sessionID % sessionMapNum]
	smap.RLock()
	defer smap.RUnlock()
	session, _ := smap.sessions[sessionID]
	return session
}

// 查找所有远端为address客户端
func (this *Manager) GetSessionByAddress(address string)  (conns []*TcpConnection) {
	this.BroadcastRun(func (conn *TcpConnection) {
		if conn.Address == address {
			conns = append(conns, conn)
		}
	})

	return conns
}

//通过轮训的方式找到hash
func (this *Manager) GetRotationSession() *TcpConnection {
	var all_conns []*TcpConnection

	this.BroadcastRun(func (conn *TcpConnection){
		all_conns = append(all_conns, conn)
	})

	if len(all_conns) == 0 {
		return nil
	}

	index := (this.current) % uint64(len(all_conns))
	this.current++

	return all_conns[index]
}

func (this *Manager) GetRandomSession() *TcpConnection {
	//每次使用当前时间做种子；
	index := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(sessionMapNum)
	//log.Println("GetRandomSession : ", index, "    ", this.sessionMaps)

	//随机选择一个数组Map开始循环，总共执行sessionMapNum次
	for i := 0; i < sessionMapNum; i++ {
		index++
		if index >= sessionMapNum {
			index = 0
		}

		smap := &this.sessionMaps[index]
		smap.RLock()
		conn := smap.RandomSelectFromMap()
		if conn != nil {
			smap.RUnlock()
			return conn
		}
		smap.RUnlock()
	}

	return nil
}

//TODO 暂时先不实现
func (this *Manager) GetHashSession(hashCode uint64) *TcpConnection {
	return nil
}

//全局Connection共同执行一个函数
func (this *Manager) BroadcastRun(handler func(*TcpConnection)) {
	for i := 0; i < sessionMapNum; i++ {
		smap := &this.sessionMaps[i]
		//TODO: 注意这个地方的并发安全问题
		smap.RLock()
		for _, v := range smap.sessions {
			handler(v)
		}
		smap.RUnlock()
	}
}

func (this *Manager) PutSession(session *TcpConnection) {
	smap := &this.sessionMaps[session.ConnID % sessionMapNum]
	smap.Lock()
	defer smap.Unlock()
	smap.sessions[session.ConnID] = session
	this.disposeWait.Add(1)
}

func (this *Manager) delSession(session *TcpConnection) {
	if this.disposeFlag {
		this.disposeWait.Done()
		return
	}
	smap := &this.sessionMaps[session.ConnID % sessionMapNum]

	smap.Lock()
	defer smap.Unlock()
	delete(smap.sessions, session.ConnID)
	this.disposeWait.Done()
}

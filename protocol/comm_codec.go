package protocol

import (
	"net"
	"io"
	log "github.com/sotter/dovenet/log"
	"fmt"
	"encoding/binary"
	"bytes"
)

type CommCodec struct {
	TcpConn net.Conn
}

func NewCommCodec(tcpConn net.Conn) *CommCodec {
	return &CommCodec {
		TcpConn : tcpConn,
	}
}

type CommSplitHeader struct {
	MagicNumber uint32      // 第一个比较为一个魔数，用于分包标记
	MsgType     uint16      // 消息类型
	Length      uint32      // 第二个字段为长度
}

func (this *CommSplitHeader) Decode(buffer []byte) error {
	if len(buffer) < 10 {
		return fmt.Errorf("Decode CommSplitHeader, buffer len is not enough !!! ")
	}

	this.MagicNumber = binary.BigEndian.Uint32(buffer[0:4])
	this.MsgType = binary.BigEndian.Uint16(buffer[4:6])
	this.Length = binary.BigEndian.Uint32(buffer[6:10])

	return nil
}

//把解包后的结果放入到buffer中
func (this *CommSplitHeader)Encode() []byte  {
	buffer := make([]byte, 10)
	binary.BigEndian.PutUint32(buffer[0:], this.MagicNumber)
	binary.BigEndian.PutUint16(buffer[4:], this.MsgType)
	binary.BigEndian.PutUint32(buffer[6:], this.Length)

	return buffer
}

const GMagicNumber = 0x132afabd

type CommMsg struct {
	Header CommSplitHeader
	Body   []byte
}

func NewCommMsg(msg_type uint16, body []byte) *CommMsg{
	return &CommMsg {
		Header : CommSplitHeader {
			MagicNumber : GMagicNumber,
			MsgType  : msg_type,
			Length   : uint32(len(body)),
		},
		Body : body,
	}
}

func (this *CommMsg)Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	//一次把内存分配够
	buf.Grow(10 + int(this.Header.Length))
	buf.Write(this.Header.Encode())
	buf.Write(this.Body)
	return buf.Bytes(), nil
}

func (this *CommCodec) Read() (msg Message, e error)  {
	for {
		head := make([]byte, 10)
		_, err := io.ReadFull(this.TcpConn, head)
		if err != nil {
			log.Println("WafConn Err :", err.Error())
			return nil, err
		}

		var msg CommMsg
		msg.Header.Decode(head)

		//如果msg_type == 0是心跳包，直接原路返回
		if msg.Header.MsgType == uint16(0) {
			this.Write(&msg)
			return nil, nil
		}

		pdubuf := make([]byte, msg.Header.Length)
		_, err = io.ReadFull(this.TcpConn, pdubuf)

		if err != nil {
			log.Println("CommCodec Read Err :", err.Error())
			return nil, err
		}

		msg.Body = pdubuf
		return &msg, nil
	}
}

func (this *CommCodec)Write(msg Message) (n int, err error) {
	buffer, e := msg.Serialize()
	if e != nil {
		return 0, err
	}
	return this.TcpConn.Write(buffer)
}

func (this *CommCodec)WriteBinary(msg []byte) (n int, err error) {
	return this.TcpConn.Write(msg)
}

//发送心跳包 DoHeartBeat
func (this *CommCodec)DoHeartBeat() error {
	ht := &CommMsg {
		Header : CommSplitHeader {
			MagicNumber : GMagicNumber,
			MsgType  : uint16(0),
			Length   : uint32(0),
		},
	}
	_, err := this.Write(ht)
	return err
}

func (this *CommCodec)Close() error {
	return this.TcpConn.Close()
}
package sdk

import (
	"encoding/json"
	"fmt"
	"net"
	"platoIM/common/idl/message"
	"platoIM/common/tcp"
	"sync/atomic"

	"google.golang.org/protobuf/proto"
)

type connect struct {
	sendChan, recvChan chan *Message
	conn               *net.TCPConn
	connID             uint64
	ip                 net.IP
	port               int
}

func newConnect(ip net.IP, port int) *connect {
	clientConn := &connect{
		sendChan: make(chan *Message),
		recvChan: make(chan *Message),
		ip:       ip,
		port:     port,
	}
	addr := &net.TCPAddr{
		IP:   ip,
		Port: port,
	}
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		fmt.Printf("DialTCP.err=%+v", err)
		return nil
	}
	clientConn.conn = conn
	return clientConn
}

func handPushMsg(c *connect, data []byte) *Message {
	pushMsg := &message.PushMsg{}
	proto.Unmarshal(data, pushMsg)
	msg := &Message{}
	json.Unmarshal(pushMsg.Content, msg)
	ackMsg := &message.ACKMsg{
		Type:   message.CmdType_UP,
		ConnID: c.connID,
	}
	ackData, _ := proto.Marshal(ackMsg)
	c.send(message.CmdType_ACK, ackData)
	return msg
}

func handAckMsg(c *connect, payload []byte) *Message {
	ackMsg := &message.ACKMsg{}
	proto.Unmarshal(payload, ackMsg)
	switch ackMsg.Type {
	case message.CmdType_Login, message.CmdType_ReConn:
		atomic.StoreUint64(&c.connID, ackMsg.ConnID)
	}
	return &Message{
		Type:       MsgTypeAck,
		Name:       "plato",
		FormUserID: "1212121",
		ToUserID:   "222212122",
		Content:    ackMsg.Msg,
	}
}

func (c *connect) send(ty message.CmdType, Payload []byte) error {
	// 直接发送给接收方
	msgCmd := message.MsgCmd{
		Type:    ty,
		Payload: Payload,
	}
	msg, err := proto.Marshal(&msgCmd)
	if err != nil {
		panic(err)
	}
	dataPgk := tcp.DataPgk{
		Data: msg,
		Len:  uint32(len(msg)),
	}
	_, err = c.conn.Write(dataPgk.Marshal())
	return err
}

func (c *connect) reConn() {
	c.conn.Close()
	addr := &net.TCPAddr{
		IP:   c.ip,
		Port: c.port,
	}
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		fmt.Printf("DialTCP.err=%+v", err)
	}
	c.conn = conn
}

func (c *connect) recv() <-chan *Message {
	return c.recvChan
}

func (c *connect) close() {
	// 目前没啥值得回收的
	c.conn.Close()
}

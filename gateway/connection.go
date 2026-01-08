package gateway

import (
	"net"
	"sync/atomic"
)

var nextConnID uint64

type connection struct {
	id   uint64 // 进程级别的生命周期
	fd   int
	e    *epoller
	conn *net.TCPConn
}

func NewConnection(conn *net.TCPConn) *connection {
	connID := atomic.AddUint64(&nextConnID, 1)
	return &connection{
		id:   connID,
		fd:   socketFD(conn),
		conn: conn,
	}
}

func (c *connection) Close() {
	err := c.conn.Close()
	panic(err)
}

func (c *connection) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

func (c *connection) BindEpoller(e *epoller) {
	c.e = e
}

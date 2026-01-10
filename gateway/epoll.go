// Package gateway 实现了基于 epoll 的高性能网络 I/O 多路复用
// 本包主要实现以下功能：
// 1. 使用 epoll 实现高效的 TCP 连接管理
// 2. 支持百万级并发连接
// 3. 实现连接数限制和优雅关闭
// 4. 采用多 worker 协程处理连接事件
package gateway

import (
	"fmt"
	"log"
	"net"
	"platoIM/common/config"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/unix"
)

// 全局对象
var ep *ePool    // epoll 连接池，用于管理所有 TCP 连接
var tcpNum int32 // 当前服务器的 TCP 连接数量

// ePool 表示 epoll 连接池，用于管理和调度所有 TCP 连接
type ePool struct {
	eChan  chan *connection // 新连接通道
	tables sync.Map         // 连接 ID 到连接对象的映射表
	eSize  int              // epoll 实例数量
	done   chan struct{}    // 关闭信号通道

	ln *net.TCPListener                 // TCP 监听器
	f  func(c *connection, ep *epoller) // 连接处理回调函数
}

// initEpoll 初始化 epoll 连接池
// ln: TCP 监听器
// f: 连接处理回调函数
func initEpoll(ln *net.TCPListener, f func(c *connection, ep *epoller)) {
	setLimit()
	ep = newEPool(ln, f)
	ep.createAcceptProcess()
	ep.startEPool()
}

// newEPool 创建新的 epoll 连接池
// ln: TCP 监听器
// cb: 连接处理回调函数
func newEPool(ln *net.TCPListener, cb func(c *connection, ep *epoller)) *ePool {
	return &ePool{
		eChan:  make(chan *connection, config.GetGatewayEpollerChanNum()),
		done:   make(chan struct{}),
		eSize:  config.GetGatewayEpollerNum(),
		tables: sync.Map{},
		ln:     ln,
		f:      cb,
	}
}

// createAcceptProcess 创建 Accept 处理协程
// 根据 CPU 核心数创建对应数量的协程处理新连接
func (e *ePool) createAcceptProcess() {
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for {
				conn, e := e.ln.AcceptTCP()
				// 限流熔断
				if !checkTcp() {
					_ = conn.Close()
					continue
				}
				setTcpConfig(conn) // 设置 TCP 连接参数
				if e != nil {
					if ne, ok := e.(net.Error); ok && ne.Temporary() {
						fmt.Errorf("accept temp err: %v", ne)
						continue
					}
					fmt.Errorf("accept err: %v", e)
				}
				c := NewConnection(conn) // 创建新的连接对象
				ep.addTask(c)            // 添加到任务队列
			}
		}()
	}
}

// addTask 添加新连接到任务队列
func (e *ePool) addTask(c *connection) {
	e.eChan <- c
}

// startEPool 启动 epoll 处理协程池
// 创建多个 epoll 实例，每个实例由一个协程处理
func (e *ePool) startEPool() {
	for i := 0; i < e.eSize; i++ {
		go e.startEProc()
	}
}

// startEProc 启动单个 epoll 处理协程
// 包含两个主要功能：
// 1. 处理新连接的加入
// 2. 处理已有连接的 I/O 事件
func (e *ePool) startEProc() {
	ep, err := newEpoller()
	if err != nil {
		panic(err)
	}
	// 处理新连接
	go func() {
		for {
			select {
			case <-e.done:
				return
			case conn := <-e.eChan:
				addTcpNum()
				fmt.Printf("tcpNum:%d\n", tcpNum)
				if err := ep.add(conn); err != nil {
					fmt.Printf("failed to add connection %v\n", err)
					conn.Close() //登录未成功直接关闭连接
					continue
				}
				fmt.Printf("EpollerPool new connection[%v] tcpSize:%d\n", conn.RemoteAddr(), tcpNum)
			}
		}
	}()
	// 处理 I/O 事件
	for {
		select {
		case <-e.done:
			return
		default:
			// 使用 200ms 超时避免忙轮询
			connections, err := ep.wait(200)
			if err != nil && err != syscall.EINTR {
				fmt.Printf("failed to epoll wait %v\n", err)
				continue
			}
			for _, conn := range connections {
				if conn == nil {
					break
				}
				e.f(conn, ep) // 调用回调处理连接
			}
		}
	}
}

// epoller 表示一个 epoll 实例
type epoller struct {
	fd            int      // epoll 文件描述符
	fdToConnTable sync.Map // 文件描述符到连接对象的映射表
}

// add 添加连接到 epoll 实例
// 默认使用水平触发模式，TODO: 可优化为边缘触发模式
func (e *epoller) add(conn *connection) error {
	fd := conn.fd
	// 设置 EPOLLIN(可读) 和 EPOLLHUP(连接关闭) 事件
	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_ADD, fd, &unix.EpollEvent{Events: unix.EPOLLIN | unix.EPOLLHUP, Fd: int32(fd)})
	if err != nil {
		return err
	}
	e.fdToConnTable.Store(conn.fd, conn)
	ep.tables.Store(conn.id, conn)
	conn.BindEpoller(e)
	return nil
}

// wait 等待 epoll 事件
// msec: 超时时间(毫秒)
// 返回发生事件的连接列表
func (e *epoller) wait(msec int) ([]*connection, error) {
	events := make([]unix.EpollEvent, config.GetGatewayEpollWaitQueueSize())
	n, err := unix.EpollWait(e.fd, events, msec)
	if err != nil {
		return nil, err
	}
	var connections []*connection
	for i := 0; i < n; i++ {
		if conn, ok := e.fdToConnTable.Load(int(events[i].Fd)); ok {
			connections = append(connections, conn.(*connection))
		}
	}
	return connections, nil
}

// remove 从 epoll 实例中移除连接
func (e *epoller) remove(c *connection) error {
	subTcpNum()
	fd := c.fd
	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_DEL, fd, nil)
	if err != nil {
		return err
	}
	ep.tables.Delete(c.id)
	e.fdToConnTable.Delete(c.fd)
	return nil
}

// socketFD 获取 TCP 连接的文件描述符
func socketFD(conn *net.TCPConn) int {
	tcpConn := reflect.Indirect(reflect.ValueOf(*conn)).FieldByName("conn")
	fdVal := tcpConn.FieldByName("fd")
	pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")
	return int(pfdVal.FieldByName("Sysfd").Int())
}

func newEpoller() (*epoller, error) {
	fd, err := syscall.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &epoller{
		fd: fd,
	}, nil
}

// setLimit 设置进程打开文件数限制
// 将软限制设置为与硬限制相同
func setLimit() {
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}

	log.Printf("set cur limit: %d", rLimit.Cur)
}

// checkTcp 检查是否超过最大连接数限制
func checkTcp() bool {
	num := getTcpNum()
	maxTcpNum := config.GetGatewayMaxTcpNum()
	return num <= maxTcpNum
}

// addTcpNum 增加 TCP 连接计数
func addTcpNum() {
	atomic.AddInt32(&tcpNum, 1)
}

// getTcpNum 获取当前 TCP 连接数
func getTcpNum() int32 {
	return atomic.LoadInt32(&tcpNum)
}

// subTcpNum 减少 TCP 连接计数
func subTcpNum() {
	atomic.AddInt32(&tcpNum, -1)
}

// setTcpConifg 设置 TCP 连接参数
// 启用 TCP keepalive
func setTcpConfig(c *net.TCPConn) {
	_ = c.SetKeepAlive(true)
}

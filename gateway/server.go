// Package gateway 实现了基于高性能网络 I/O 多路复用的网关服务
// 本包中的 server.go 文件主要实现以下功能：
// 1. 提供网关服务的启动和初始化
// 2. 实现 TCP 连接的数据读写处理
// 3. 支持 RPC 服务的注册和调用
// 4. 实现命令处理和消息推送
package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"platoIM/common/config"
	"platoIM/common/prpc"
	"platoIM/common/tcp"
	"platoIM/gateway/rpc/client"
	"platoIM/gateway/rpc/service"

	"google.golang.org/grpc"
)

// cmdChannel 用于处理命令的通道
// 包括连接关闭和消息推送等命令
var cmdChannel chan *service.CmdContext

// RunMain 启动网关服务
// 1. 初始化配置
// 2. 启动 TCP 监听
// 3. 初始化工作池和 epoll
// 4. 注册 RPC 服务
// 5. 启动命令处理协程
func RunMain(path string) {
	// 初始化配置文件
	config.Init(path)
	// 启动 TCP 监听
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{
		Port: config.GetGatewayTCPServerPort(),
	})
	if err != nil {
		log.Fatalf("StartTCPEpollServer err:%s", err.Error())
		panic(err)
	}
	// 初始化工作池
	initWorkPool()
	// 初始化 epoll
	initEpoll(ln, runProc)
	fmt.Println("-------------im gateway started-------------")
	// 初始化命令通道
	cmdChannel = make(chan *service.CmdContext, config.GetGatewayCmdChannelNum())
	// 创建 RPC 服务器实例
	s := prpc.NewPServer(
		prpc.WithServiceName(config.GetGatewayServiceName()),
		prpc.WithIP(config.GetGatewayServiceAddr()),
		prpc.WithPort(config.GetGatewayRPCServerPort()), prpc.WithWeight(config.GetGatewayRPCWeight()))
	fmt.Println(config.GetGatewayServiceName(), config.GetGatewayServiceAddr(), config.GetGatewayRPCServerPort(), config.GetGatewayRPCWeight())
	// 注册 RPC 服务
	s.RegisterService(func(server *grpc.Server) {
		service.RegisterGatewayServer(server, &service.Service{CmdChannel: cmdChannel})
	})
	// 启动 RPC 客户端
	client.Init()
	// 启动命令处理协程
	go cmdHandler()
	// 启动 RPC 服务器
	s.Start(context.TODO())
}

// runProc 处理单个连接的数据读取和业务逻辑
// 1. 读取完整的消息包
// 2. 提交到工作池处理
// 3. 处理连接异常和关闭
func runProc(c *connection, ep *epoller) {
	ctx := context.Background() // 起始的context
	// 读取一个完整的消息包
	dataBuf, err := tcp.ReadData(c.conn)
	if err != nil {
		// 如果读取连接时发现连接关闭，则直接断开连接
		// 通知 state 清理掉意外退出的连接的状态信息
		if errors.Is(err, io.EOF) {
			// 这步操作是异步的，不需要等到返回成功再进行，因为消息可靠性的保障是通过协议完成的而非某次 cmd
			ep.remove(c)
			client.CancelConn(&ctx, getEndpoint(), c.id, nil)
		}
		return
	}
	// 提交到工作池处理
	err = wPool.Submit(func() {
		// 交给 state server rpc 处理
		client.SendMsg(&ctx, getEndpoint(), c.id, dataBuf)
	})
	if err != nil {
		fmt.Errorf("runProc:err:%+v\n", err.Error())
	}
}

// cmdHandler 处理命令通道中的命令
// 支持两种命令类型：
// 1. DelConnCmd: 关闭连接
// 2. PushCmd: 推送消息
func cmdHandler() {
	for cmd := range cmdChannel {
		// 异步提交到协池中完成发送任务
		switch cmd.Cmd {
		case service.DelConnCmd:
			wPool.Submit(func() { closeConn(cmd) })
		case service.PushCmd:
			wPool.Submit(func() { sendMsgByCmd(cmd) })
		default:
			panic("command undefined")
		}
	}
}

// closeConn 关闭指定的连接
// 根据连接 ID 查找并关闭连接
func closeConn(cmd *service.CmdContext) {
	// 根据连接 ID 查找连接
	if connPtr, ok := ep.tables.Load(cmd.ConnID); ok {
		conn, _ := connPtr.(*connection)
		conn.Close()
		ep.tables.Delete(cmd.ConnID)
	}
}

// sendMsgByCmd 向指定连接发送消息
// 1. 根据连接 ID 查找连接
// 2. 封装消息包
// 3. 发送数据
func sendMsgByCmd(cmd *service.CmdContext) {
	// 根据连接 ID 查找连接
	if connPtr, ok := ep.tables.Load(cmd.ConnID); ok {
		conn, _ := connPtr.(*connection)
		// 封装消息包
		dp := tcp.DataPgk{
			Len:  uint32(len(cmd.Payload)),
			Data: cmd.Payload,
		}
		tcp.SendData(conn.conn, dp.Marshal())
	}
}

// getEndpoint 获取网关服务的 RPC 端点地址
// 返回格式：IP:Port
func getEndpoint() string {
	return fmt.Sprintf("%s:%d", config.GetGatewayServiceAddr(), config.GetGatewayRPCServerPort())
}

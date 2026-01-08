package state

import (
	"context"
	"fmt"
	"platoIM/common/config"
	"platoIM/common/idl/message"
	"platoIM/common/prpc"
	"platoIM/state/rpc/client"
	"platoIM/state/rpc/service"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
)

// RunMain 启动网关服务
func RunMain(path string) {
	config.Init(path)
	cmdChannel = make(chan *service.CmdContext, config.GetStateCmdChannelNum())

	s := prpc.NewPServer(
		prpc.WithServiceName(config.GetStateServiceName()),
		prpc.WithIP(config.GetStateServiceAddr()),
		prpc.WithPort(config.GetStateServerPort()), prpc.WithWeight(config.GetStateRPCWeight()))

	s.RegisterService(func(server *grpc.Server) {
		service.RegisterStateServer(server, &service.Service{CmdChannel: cmdChannel})
	})
	// 初始化RPC 客户端
	client.Init()
	// 启动 命令处理写协程
	go cmdHandler()
	// 启动 rpc server
	s.Start(context.TODO())
}

func cmdHandler() {
	for cmd := range cmdChannel {
		switch cmd.Cmd {
		case service.CancelConnCmd:
			fmt.Printf("cancel conn endpoint:%s, fd:%d, data:%+v", cmd.Endpoint, cmd.ConnID, cmd.Payload)
		case service.SendMsgCmd:
			fmt.Println("cmdHandler", string(cmd.Payload))
			client.Push(cmd.Ctx, cmd.ConnID, cmd.Payload)
		}
	}
}

func heartbeatMsgHandler(cmdCtx *service.CmdContext, msgCmd *message.MsgCmd) {
	heartMsg := &message.HeartbeatMsg{}
	err := proto.Unmarshal(msgCmd.Payload, heartMsg)
	if err != nil {
		fmt.Printf("hearbeatMsgHandler:err=%s\n", err.Error())
		return
	}
	cs.reSetHeartTimer(cmdCtx.ConnID)
}

func loginMsgHandler(cmdCtx *service.CmdContext, msgCmd *message.MsgCmd) {
	loginMsg := &message.LoginMsg{}
	err := proto.Unmarshal(msgCmd.Payload, loginMsg)
	if err != nil {
		fmt.Printf("loginMsgHandler:err=%s\n", err.Error())
		return
	}
	if loginMsg.Head != nil {
		// 这里会把 login msg 传送给业务层做处理
		fmt.Println("loginMsgHandler", loginMsg.Head.DeviceID)
	}
	t := AfterFunc(300*time.Second, func() {
		clearState(cmdCtx.ConnID)
	})
	connToStateTable.Store(cmdCtx.ConnID, &connState{
		heartTimer: t,
		connID:     cmdCtx.ConnID,
	})
	sendACKMsg(cmdCtx.ConnID, 0, "login")
}

func sendACKMsg(connID uint64, code uint32, msg string) {
	ackMsg := &message.ACKMsg{}
	ackMsg.Code = code
	ackMsg.Msg = msg
	ackMsg.ConnID = connID
	ctx := context.TODO()
	downLoad, err := proto.Marshal(ackMsg)
	if err != nil {
		fmt.Printf("send ACK Msg:err=%s\n", err.Error())
	}
	mc := &message.MsgCmd{}
	mc.Type = message.CmdType_ACK
	mc.Payload = downLoad
	data, err := proto.Marshal(mc)
	if err != nil {
		fmt.Printf("send ACK Msg:err=%s\n", err.Error())
	}
	client.Push(&ctx, connID, data)
}

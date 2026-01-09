package state

import (
	"context"
	"fmt"
	"platoIM/common/config"
	"platoIM/common/idl/message"
	"platoIM/common/prpc"
	"platoIM/state/rpc/client"
	"platoIM/state/rpc/service"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
)

// RunMain 启动网关服务
func RunMain(path string) {
	// 启动时的全局上下文
	ctx := context.TODO()
	// 初始化全局配置
	config.Init(path)
	// 初始化RPC 客户端
	client.Init()
	// 启动时间轮
	InitTimer()
	// 启动远程cache状态机组件
	InitCacheState(ctx)
	// 启动 命令处理写协程
	go cmdHandler()
	// 注册rpc server
	s := prpc.NewPServer(
		prpc.WithServiceName(config.GetStateServiceName()),
		prpc.WithIP(config.GetStateServiceAddr()),
		prpc.WithPort(config.GetStateServerPort()), prpc.WithWeight(config.GetStateRPCWeight()))

	s.RegisterService(func(server *grpc.Server) {
		service.RegisterStateServer(server, cs.server)
	})
	// 启动 rpc server
	s.Start(ctx)
}

// 消费信令通道，识别gateway与state server之间的协议路由
func cmdHandler() {
	for cmdCtx := range cs.server.CmdChannel {
		switch cmdCtx.Cmd {
		case service.CancelConnCmd:
			fmt.Printf("cancel conn endpoint:%s, coonID:%d, data:%+v\n", cmdCtx.Endpoint, cmdCtx.ConnID, cmdCtx.Payload)
			cs.connLogOut(*cmdCtx.Ctx, cmdCtx.ConnID)
		case service.SendMsgCmd:
			msgCmd := &message.MsgCmd{}
			err := proto.Unmarshal(cmdCtx.Payload, msgCmd)
			if err != nil {
				fmt.Printf("SendMsgCmd:err=%s\n", err.Error())
			}
			msgCmdHandler(cmdCtx, msgCmd)
		}
	}
}

// 识别消息类型，识别客户端与state server之间的协议路由
func msgCmdHandler(cmdCtx *service.CmdContext, msgCmd *message.MsgCmd) {
	switch msgCmd.Type {
	case message.CmdType_Login:
		loginMsgHandler(cmdCtx, msgCmd)
	case message.CmdType_Heartbeat:
		heartbeatMsgHandler(cmdCtx, msgCmd)
	case message.CmdType_ReConn:
		reConnMsgHandler(cmdCtx, msgCmd)
	case message.CmdType_UP:
		upMsgHandler(cmdCtx, msgCmd)
	case message.CmdType_ACK:
		ackMsgHandler(cmdCtx, msgCmd)
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
	err = cs.connLogin(*cmdCtx.Ctx, loginMsg.Head.DeviceID, cmdCtx.ConnID)
	if err != nil {
		panic(err)
	}
	sendACKMsg(message.CmdType_Login, cmdCtx.ConnID, 0, 0, "login ok")
}

func reConnMsgHandler(cmdCtx *service.CmdContext, msgCmd *message.MsgCmd) {
	reConnMsg := &message.ReConnMsg{}
	err := proto.Unmarshal(msgCmd.Payload, reConnMsg)
	var code uint32
	msg := "re conn ok"
	if err != nil {
		fmt.Printf("reConnMsgHandler:err=%s\n", err.Error())
		return
	}
	if err := cs.reConn(*cmdCtx.Ctx, reConnMsg.Head.ConnID, cmdCtx.ConnID); err != nil {
		code, msg = 1, "record failed"
		panic(err)
	}
	sendACKMsg(message.CmdType_ReConn, cmdCtx.ConnID, 0, code, msg)
}

func upMsgHandler(cmdCtx *service.CmdContext, msgCmd *message.MsgCmd) {
	upMsg := &message.UPMsg{}
	err := proto.Unmarshal(msgCmd.Payload, upMsg)
	if err != nil {
		fmt.Printf("upMsgHandler:err=%s\n", err.Error())
		return
	}
	if cs.compareAndIncrClientID(*cmdCtx.Ctx, cmdCtx.ConnID, upMsg.Head.ClientID, upMsg.Head.SessionId) {
		// 调用下游业务层rpc，只有当rpc回复成功后才能更新max_clientID
		sendACKMsg(message.CmdType_UP, cmdCtx.ConnID, upMsg.Head.ClientID, 0, "up ok")
		// TODO 这里应该调用业务层的代码
		pushMsg(*cmdCtx.Ctx, cmdCtx.ConnID, cs.msgID, 0, upMsg.UPMsgBody)
	}
}

// 由业务层调用，下行消息处理
func pushMsg(ctx context.Context, connID, sessionID, msgID uint64, data []byte) {
	// TODO 先在这里push消息
	pushMsg := &message.PushMsg{
		MsgID:   cs.msgID,
		Content: data,
	}
	if data, err := proto.Marshal(pushMsg); err != nil {
		fmt.Printf("Marshal:err=%s\n", err.Error())
	} else {
		//TODO 这里就要涉及到 下行消息的下发了,不管成功与否，都要更新last msg
		sendMsg(connID, message.CmdType_Push, data)
		err = cs.appendLastMsg(ctx, connID, pushMsg)
		if err != nil {
			panic(err)
		}
	}
}

func ackMsgHandler(cmdCtx *service.CmdContext, msgCmd *message.MsgCmd) {
	ackMsg := &message.ACKMsg{}
	err := proto.Unmarshal(msgCmd.Payload, ackMsg)
	if err != nil {
		fmt.Printf("ackMsgHandler:err=%s\n", err.Error())
		return
	}
	cs.ackLastMsg(*cmdCtx.Ctx, ackMsg.ConnID, ackMsg.SessionID, ackMsg.MsgID)
}

// 发送ack msg
func sendACKMsg(ackType message.CmdType, connID, clientID uint64, code uint32, msg string) {
	ackMsg := &message.ACKMsg{}
	ackMsg.Code = code
	ackMsg.Msg = msg
	ackMsg.ConnID = connID
	ackMsg.Type = ackType
	ackMsg.ClientID = clientID
	downLoad, err := proto.Marshal(ackMsg)
	if err != nil {
		fmt.Println("sendACKMsg", err)
	}
	sendMsg(connID, message.CmdType_ACK, downLoad)
}

func sendMsg(connID uint64, ty message.CmdType, downLoad []byte) {
	mc := &message.MsgCmd{}
	mc.Type = ty
	mc.Payload = downLoad
	data, err := proto.Marshal(mc)
	ctx := context.TODO()
	if err != nil {
		fmt.Println("sendMsg", ty, err)
	}
	client.Push(&ctx, connID, data)
}

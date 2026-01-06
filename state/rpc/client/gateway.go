package client

import (
	"context"
	"fmt"
	"platoIM/common/config"
	"platoIM/common/prpc"
	"platoIM/gateway/rpc/service"
	"time"
)

var gatewayClient service.GatewayClient

func initGatewayClient() {
	pCli, err := prpc.NewPClient(config.GetGatewayServiceName())
	if err != nil {
		panic(err)
	}
	gatewayClient = service.NewGatewayClient(pCli.Conn())
}

func DelConn(ctx *context.Context, fd int32, payLoad []byte) error {
	rpcCtx, _ := context.WithTimeout(*ctx, 100*time.Millisecond)
	gatewayClient.DelConn(rpcCtx, &service.GatewayRequest{Fd: fd, Data: payLoad})
	return nil
}

func Push(ctx *context.Context, fd int32, payLoad []byte) error {
	rpcCtx, _ := context.WithTimeout(*ctx, 100*time.Second)
	resp, err := gatewayClient.Push(rpcCtx, &service.GatewayRequest{Fd: fd, Data: payLoad})
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(resp)
	return nil
}

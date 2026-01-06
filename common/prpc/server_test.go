package prpc

import (
	"context"
	"testing"

	"platoIM/common/config"

	"platoIM/common/prpc/example/helloservice"

	ptrace "platoIM/common/prpc/trace"

	"google.golang.org/grpc"
)

const (
	testIp   = "127.0.0.1"
	testPort = 8867
)

func TestNewPServer(t *testing.T) {
	config.Init("../../plato.yaml")

	ptrace.StartAgent()
	defer ptrace.StopAgent()

	s := NewPServer(WithServiceName("plato_server"), WithIP(testIp), WithPort(testPort), WithWeight(100))
	s.RegisterService(func(server *grpc.Server) {
		helloservice.RegisterGreeterServer(server, helloservice.HelloServer{})
	})
	s.Start(context.TODO())
}

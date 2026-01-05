package discovery

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestServiceRegiste(t *testing.T) {
	viper.Set("discovery.endpoints", []string{"127.0.0.1:2379"})
	viper.Set("discovery.timeout", 5) // 设置超时时间为 5 (配合代码里的 * time.Second)
	ctx := context.Background()
	ser, err := NewServiceRegister(&ctx, "/web/node1", &EndpointInfo{
		IP:   "127.0.0.1",
		Port: "9999",
	}, 5)
	if err != nil {
		log.Fatalln(err)
	}
	//监听续租相应chan
	go ser.ListenLeaseRespChan()
	select {
	case <-time.After(20 * time.Second):
		ser.Close()
	}
}

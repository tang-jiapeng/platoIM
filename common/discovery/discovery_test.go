package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestServiceDiscovery(t *testing.T) {
	viper.Set("discovery.endpoints", []string{"127.0.0.1:2379"})
	viper.Set("discovery.timeout", 5)

	ctx := context.Background()

	ser := NewServiceDiscovery(&ctx)
	defer ser.Close()

	// 让监听任务在后台运行，不阻塞主线程
	go ser.WatchService("/web/", func(key, value string) {
		t.Logf("Detected change: key=%s, value=%s", key, value)
	}, func(key, value string) {})

	go ser.WatchService("/gRPC/", func(key, value string) {
		t.Logf("Detected change: key=%s, value=%s", key, value)
	}, func(key, value string) {})

	t.Log("Watcher started, waiting for 5 seconds...")

	//  运行 20 秒钟后自动结束
	select {
	case <-time.After(20 * time.Second):
		t.Log("Test finished automatically.")
	}
}

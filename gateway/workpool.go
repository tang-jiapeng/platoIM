// Package gateway 实现了基于高性能网络 I/O 多路复用的网关服务
// 本包中的 workpool.go 文件主要实现以下功能：
// 1. 提供全局工作池管理
// 2. 使用 ants 包实现高效的协程池
// 3. 支持可配置的工作池大小
// 4. 优化协程资源利用
package gateway

import (
	"fmt"
	"platoIM/common/config"

	"github.com/panjf2000/ants"
)

// wPool 全局工作池实例
// 使用 ants 包提供的协程池实现
var wPool *ants.Pool

// initWorkPoll 初始化工作池
// 根据配置创建指定大小的协程池
// 使用 ants 包实现协程复用，避免频繁创建销毁协程
func initWorkPool() {
	var err error
	if wPool, err = ants.NewPool(config.GetGatewayWorkerPoolNum()); err != nil {
		fmt.Printf("InitWorkPoll.err :%s num:%d\n", err.Error(), config.GetGatewayWorkerPoolNum())
	}
}

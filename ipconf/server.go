// Package ipconf 实现了基于多策略的 IP 配置和调度服务
// 本包中的 server.go 文件主要实现以下功能：
// 1. 提供 HTTP 服务器的启动和初始化
// 2. 集成 Hertz 框架实现高性能 Web 服务
// 3. 注册 API 路由和处理函数
// 4. 管理服务组件的生命周期
package ipconf

import (
	"platoIM/common/config"
	"platoIM/ipconf/domain"
	"platoIM/ipconf/source"

	"github.com/cloudwego/hertz/pkg/app/server"
)

// RunMain 启动web容器
// RunMain 启动 IP 配置服务
// 主要流程：
// 1. 初始化配置
// 2. 启动数据源服务
// 3. 初始化调度层
// 4. 配置 HTTP 路由
// 5. 启动 Web 服务器
func RunMain(path string) {
	config.Init(path)
	source.Init() //数据源优先启动
	domain.Init() // 初始化调度层
	s := server.Default(server.WithHostPorts(":6789"))
	s.GET("/ip/list", GetIpInfoList)
	s.Spin()
}

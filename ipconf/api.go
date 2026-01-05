// Package ipconf 实现了基于多策略的 IP 配置和调度服务
// 本包中的 api.go 文件主要实现以下功能：
// 1. 提供 HTTP API 接口层
// 2. 处理客户端请求和响应
// 3. 封装域名调度结果
// 4. 实现错误处理和恢复机制
package ipconf

import (
	"context"
	"platoIM/ipconf/domain"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// Response 定义了 API 响应的标准格式
// 包含消息、状态码和数据三个字段
type Response struct {
	Message string      `json:"message"`
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
}

// GetIpInfoList API 适配应用层
// GetIpInfoList API 处理获取 IP 信息列表的请求
// 主要流程：
// 1. 构建请求上下文
// 2. 执行 IP 调度
// 3. 返回得分最高的前5个节点
// 使用 defer-recover 机制处理可能的异常
func GetIpInfoList(c context.Context, ctx *app.RequestContext) {
	defer func() {
		if err := recover(); err != nil {
			ctx.JSON(consts.StatusBadRequest, utils.H{"err": err})
		}
	}()
	// Step0: 构建客户请求信息
	ipConfCtx := domain.BuildIpConfContext(&c, ctx)
	// Step1: 进行ip调度
	eds := domain.Dispatch(ipConfCtx)
	// Step2: 根据得分取top5返回
	ipConfCtx.AppCtx.JSON(consts.StatusOK, packRes(top5Endports(eds)))
}

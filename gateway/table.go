// Package gateway 实现了基于高性能网络 I/O 多路复用的网关服务
// 本包中的 table.go 文件主要实现以下功能：
// 1. 提供全局连接表管理
// 2. 支持设备 ID 到连接对象的映射
// 3. 使用 sync.Map 实现并发安全的表操作
package gateway

import "sync"

// tables 全局连接表实例
var tables table

// table 表示连接映射表
// 使用 sync.Map 实现线程安全的设备 ID 到连接对象的映射
type table struct {
	did2conn sync.Map // 设备 ID 到连接对象的映射表
}

// InitTables 初始化全局连接表
// 创建一个新的空连接表实例
func InitTables() {
	tables = table{
		did2conn: sync.Map{},
	}
}

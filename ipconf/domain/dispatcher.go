package domain

import (
	"platoIM/ipconf/source"
	"sort"
	"sync"
)

// Dispatcher 节点调度器，负责管理和调度可用的节点
//
// 动静结合排序策略：
// 1. 动态分数(ActiveScore)：基于节点的实时负载情况，如连接数和消息字节数
// 2. 静态分数(StaticScore)：基于节点的固有资源情况，如CPU和内存容量
// 3. 优先使用动态分数排序，分数相同时才使用静态分数
type Dispatcher struct {
	// 候选节点表，key为"ip:port"格式
	candidateTable map[string]*Endport
	// 读写锁，用于保护candidateTable的并发访问
	sync.RWMutex
}

var dp *Dispatcher

func Init() {
	dp = &Dispatcher{}
	dp.candidateTable = make(map[string]*Endport)
	go func() {
		for event := range source.EventChan() {
			switch event.Type {
			case source.AddNodeEvent:
				dp.addNode(event)
			case source.DelNodeEvent:
				dp.delNode(event)
			}
		}
	}()
}

// Dispatch 执行节点调度，实现动静结合的排序策略
//
// 排序逻辑：
// 1. 首先比较节点的动态活跃度分数(ActiveScore)
// 2. 如果动态分数相同，则比较静态资源分数(StaticScore)
// 3. 分数越高的节点排序越靠前
//
// 这种排序策略能够在保证负载均衡的同时，充分利用节点的资源能力
func Dispatch(ctx *IpConfContext) []*Endport {
	// Step1: 获得候选endport
	eds := dp.getCandidateEndport(ctx)
	// Step2: 逐一计算得分
	for _, ed := range eds {
		ed.CalculateScore(ctx)
	}
	// Step3: 全局排序，动静结合的排序策略
	sort.Slice(eds, func(i, j int) bool {
		if eds[i].ActiveScore > eds[j].ActiveScore {
			return true
		}
		if eds[i].ActiveScore == eds[j].ActiveScore {
			if eds[i].StaticScore > eds[j].StaticScore {
				return true
			}
			return false
		}
		return false
	})
	return eds
}

func (dp *Dispatcher) getCandidateEndport(ctx *IpConfContext) []*Endport {
	dp.RLock()
	defer dp.RUnlock()
	candidateList := make([]*Endport, 0, len(dp.candidateTable))
	for _, ed := range dp.candidateTable {
		candidateList = append(candidateList, ed)
	}
	return candidateList
}

func (dp *Dispatcher) delNode(event *source.Event) {
	dp.Lock()
	defer dp.Unlock()
	delete(dp.candidateTable, event.Key())
}

func (dp *Dispatcher) addNode(event *source.Event) {
	dp.Lock()
	defer dp.Unlock()
	ed := NewEndport(event.IP, event.Port)
	ed.UpdateStat(&Stat{
		ConnectNum:   event.ConnectNum,
		MessageBytes: event.MessageBytes,
	})
	dp.candidateTable[event.Key()] = ed
}

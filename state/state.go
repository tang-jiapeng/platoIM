package state

import (
	"context"
	"platoIM/common/timingwheel"
	"platoIM/state/rpc/client"
	"platoIM/state/rpc/service"
	"sync"
	"time"
)

var cmdChannel chan *service.CmdContext
var connToStateTable sync.Map

type connState struct {
	sync.RWMutex
	heartTimer   *timingwheel.Timer
	reConnTimer  *timingwheel.Timer
	msgTimer     *timingwheel.Timer
	msgTimerLock string
	connID       uint64
	did          uint64
}

func (c *connState) reSetHeartTimer() {
	c.Lock()
	defer c.Unlock()
	c.heartTimer.Stop()
	c.heartTimer = AfterFunc(5*time.Second, func() {
		c.reSetReConnTimer()
	})
}

func (c *connState) reSetReConnTimer() {
	c.Lock()
	defer c.Unlock()
	if c.reConnTimer != nil {
		c.reConnTimer.Stop()
	}
	c.reConnTimer = AfterFunc(10*time.Second, func() {
		ctx := context.TODO()
		cs.connLogOut(ctx, c.connID)
	})
}

func (c *connState) close(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()
	if c.heartTimer != nil {
		c.heartTimer.Stop()
	}
	if c.reConnTimer != nil {
		c.reConnTimer.Stop()
	}
	if c.msgTimer != nil {
		c.msgTimer.Stop()
	}
	// TODO 这里如何保证事务性，值得思考一下，或者说有没有必要保证
	// TODO 这里也可以使用lua或者pipeline 来尽可能合并两次redis的操作 通常在大规模的应用中这是有效的
	// TODO 这里是要好好思考网络调用次数的时间&空间复杂度的

	return nil
}

func clearState(connID uint64) {
	if data, ok := connToStateTable.Load(connID); ok {
		state, _ := data.(*connState)
		state.Lock()
		defer state.Unlock()
		state.reConnTimer = AfterFunc(10*time.Second, func() {
			ctx := context.TODO()
			client.DelConn(&ctx, connID, nil)
			connToStateTable.Delete(connID)
		})
	}
}

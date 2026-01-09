package state

import (
	"context"
	"fmt"
	"platoIM/common/cache"
	"platoIM/common/config"
	"platoIM/common/router"
	"platoIM/state/rpc/service"
	"strconv"
	"strings"
	"sync"
)

var cs *cacheState

type cacheState struct {
	msgID            uint64 // test
	connToStateTable sync.Map
	server           *service.Service
}

func InitCacheState(ctx context.Context) {
	cs = &cacheState{}
	cache.InitRedis(ctx)
	router.Init(ctx)
	cs.connToStateTable = sync.Map{}
	cs.initLoginSlot(ctx)
	cs.server = &service.Service{
		CmdChannel: make(chan *service.CmdContext, config.GetStateCmdChannelNum()),
	}
}

func (cs *cacheState) initLoginSlot(ctx context.Context) error {
	//loginSlotRange := config.GetStateServerLoginSlotRange()
	//for _, slot := range loginSlotRange {
	//	loginSlotKey := fmt.Sprintf(cache.LoginSlotSetKey, slot)
	//	go func() {
	//		loginSlot, err := cache.SmembersStrSlice(ctx, loginSlotKey)
	//		if err != nil {
	//			panic(err)
	//		}
	//		for _, meta := range loginSlot {
	//			did, connID := cs.loginSlotUnmarshal(meta)
	//			cs.connReLogin(ctx, did, connID)
	//		}
	//	}()
	//}
	return nil
}

func (cs *cacheState) connReLogin(ctx context.Context, did uint64, connID uint64) {
	//state := cs.newConnState(did, connID)
	//slotKey := cs.getLoginSlotKey(connID)
	//state.loadMsgTimer(ctx)
}

func (cs *cacheState) newConnState(did, connID uint64) *connState {
	state := &connState{
		connID: connID,
		did:    did,
	}
	state.reSetHeartTimer()
	return state
}

func (cs *cacheState) getLoginSlotKey(connID uint64) string {
	connStateSlotList := config.GetStateServerLoginSlotRange()
	slotSize := uint64(len(connStateSlotList))
	slot := connID % slotSize
	slotKey := fmt.Sprintf(cache.LoginSlotSetKey, connStateSlotList[slot])
	return slotKey
}

func (cs *cacheState) connLogOut(ctx context.Context, connID uint64) (uint64, error) {
	if state, ok := cs.loadConnIDState(connID); ok {
		did := state.did
		return did, state.close(ctx)
	}
	return 0, nil
}

func (cs *cacheState) reSetHeartTimer(connID uint64) {
	if state, ok := cs.loadConnIDState(connID); ok {
		state.reSetHeartTimer()
	}
}

func (cs *cacheState) loadConnIDState(connID uint64) (*connState, bool) {
	if data, ok := cs.connToStateTable.Load(connID); ok {
		sate, _ := data.(*connState)
		return sate, true
	}
	return nil, false
}

func (cs *cacheState) reConn(ctx context.Context, oldConnID uint64, newConnID uint64) error {
	var (
		did uint64
		err error
	)
	if did, err = cs.connLogOut(ctx, oldConnID); err != nil {
		return err
	}
	return cs.connLogin(ctx, did, newConnID) // 重连路由是不用更新的
}

func (cs *cacheState) connLogin(ctx context.Context, did, connID uint64) error {
	state := cs.newConnState(did, connID)
	slotKey := cs.getLoginSlotKey(connID)
	mate := cs.loginSlotMarshal(did, connID)
	err := cache.SADD(ctx, slotKey, mate)
	if err != nil {
		return err
	}

	endPoint := fmt.Sprintf("%s:%d", config.GetGatewayServiceAddr(), config.GetStateServerPort())
	err = router.AddRecord(ctx, did, endPoint, connID)
	if err != nil {
		return err
	}

	//TODO 上行消息 max_client_id 初始化, 现在相当于生命周期在conn维度，后面重构sdk时会调整到会话维度

	// 本地状态存储
	cs.storeConnIDState(connID, state)
	return nil
}

func (cs *cacheState) storeConnIDState(id uint64, state *connState) {
	cs.connToStateTable.Store(id, state)
}

func (cs *cacheState) loginSlotMarshal(did, connID uint64) string {
	return fmt.Sprintf("%d|%d", did, connID)
}

func (cs *cacheState) loginSlotUnmarshal(meta string) (uint64, uint64) {
	strs := strings.Split(meta, ",")
	if len(strs) < 2 {
		return 0, 0
	}
	did, err := strconv.ParseUint(strs[0], 10, 64)
	if err != nil {
		panic(err)
	}
	connID, err := strconv.ParseUint(strs[1], 10, 64)
	if err != nil {
		panic(err)
	}
	return did, connID
}

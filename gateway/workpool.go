package gateway

import (
	"fmt"
	"platoIM/common/config"

	"github.com/panjf2000/ants"
)

var wPool *ants.Pool

func initWorkPool() {
	var err error
	if wPool, err = ants.NewPool(config.GetGatewayWorkerPoolNum()); err != nil {
		fmt.Printf("InitWorkPoll.err :%s num:%d\n", err.Error(), config.GetGatewayWorkerPoolNum())
	}
}

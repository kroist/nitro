package gethexec

import (
	"context"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/offchainlabs/nitro/execution/gethexec/timeboost/bindings"
	"github.com/offchainlabs/nitro/util/stopwaiter"
)

type expressLaneService struct {
	stopwaiter.StopWaiter
	sync.RWMutex
	expressLaneController common.Address
	auctionContract       *bindings.ExpressLaneAuction
	initialTimestamp      time.Time
	roundDuration         time.Duration
}

func (es *expressLaneService) Start(ctxIn context.Context) {
	es.StopWaiter.Start(ctxIn, es)
	es.LaunchThread(func(ctx context.Context) {
		// Monitor for auction resolutions from the auction manager smart contract
		// and set the express lane controller for the upcoming round accordingly.
	})
}

func (es *expressLaneService) isExpressLaneTx(tx *types.Transaction) (bool, error) {
	sender, err := types.NewArbitrumSigner(nil).Sender(tx)
	if err != nil {
		return false, err
	}
	es.RLock()
	defer es.RUnlock()
	return sender == es.expressLaneController, nil
}

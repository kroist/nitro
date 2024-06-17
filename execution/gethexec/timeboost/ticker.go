package timeboost

import (
	"time"
)

type auctionCloseTicker struct {
	c                      chan time.Time
	done                   chan bool
	roundDuration          time.Duration
	auctionClosingDuration time.Duration
}

func newAuctionCloseTicker(roundDuration, auctionClosingDuration time.Duration) *auctionCloseTicker {
	return &auctionCloseTicker{
		c:                      make(chan time.Time),
		done:                   make(chan bool),
		roundDuration:          roundDuration,
		auctionClosingDuration: auctionClosingDuration,
	}
}

func (t *auctionCloseTicker) start() {
	for {
		now := time.Now()
		nextTickTime := now.Truncate(t.roundDuration).Add(t.roundDuration).Add(-t.auctionClosingDuration)
		if now.After(nextTickTime) {
			nextTickTime = nextTickTime.Add(t.roundDuration)
		}
		waitTime := nextTickTime.Sub(now)
		select {
		case <-time.After(waitTime):
			t.c <- time.Now()
		case <-t.done:
			close(t.c)
			return
		}
	}
}

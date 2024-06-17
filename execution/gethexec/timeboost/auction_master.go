package timeboost

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/log"
	"github.com/offchainlabs/nitro/execution/gethexec/timeboost/bindings"
	"github.com/pkg/errors"
)

const defaultAuctionClosingSecondsBeforeRound = 15 // Before the start of the next round.

type auctionMasterOpt func(*auctionMaster)

func WithAuctionClosingSecondsBeforeRound(d time.Duration) auctionMasterOpt {
	return func(am *auctionMaster) {
		am.auctionClosingDurationBeforeRoundStart = d
	}
}

type auctionMaster struct {
	txOpts                                 *bind.TransactOpts
	chainId                                *big.Int
	signatureDomain                        uint16
	client                                 simulated.Client
	auctionContract                        *bindings.ExpressLaneAuction
	bidsReceiver                           chan *bid
	bidCache                               *bidCache
	initialRoundTimestamp                  time.Time
	roundDuration                          time.Duration
	auctionClosingDurationBeforeRoundStart time.Duration
}

func newAuctionMaster(
	txOpts *bind.TransactOpts,
	chainId *big.Int,
	client simulated.Client,
	auctionContract *bindings.ExpressLaneAuction,
	opts ...auctionMasterOpt,
) (*auctionMaster, error) {
	initialRoundTimestamp, err := auctionContract.InitialRoundTimestamp(&bind.CallOpts{})
	if err != nil {
		return nil, err
	}
	roundDurationSeconds, err := auctionContract.RoundDurationSeconds(&bind.CallOpts{})
	if err != nil {
		return nil, err
	}
	sigDomain, err := auctionContract.BidSignatureDomainValue(&bind.CallOpts{})
	if err != nil {
		return nil, err
	}
	am := &auctionMaster{
		txOpts:                                 txOpts,
		chainId:                                chainId,
		client:                                 client,
		signatureDomain:                        sigDomain,
		auctionContract:                        auctionContract,
		bidsReceiver:                           make(chan *bid, 100),
		bidCache:                               newBidCache(),
		initialRoundTimestamp:                  time.Unix(initialRoundTimestamp.Int64(), 0),
		roundDuration:                          time.Duration(roundDurationSeconds) * time.Second,
		auctionClosingDurationBeforeRoundStart: defaultAuctionClosingSecondsBeforeRound,
	}
	for _, o := range opts {
		o(am)
	}
	return am, nil
}

func (am *auctionMaster) SubmitBid(ctx context.Context, b *bid) error {
	validated, err := am.newValidatedBid(b)
	if err != nil {
		return err
	}
	am.bidCache.add(validated)
	return nil
}

func (am *auctionMaster) Start(ctx context.Context) {
	// Receive bids in the background.
	go receiveAsync(ctx, am.bidsReceiver, am.SubmitBid)

	// Listen for sequencer health in the background and close upcoming auctions if so.
	go am.checkSequencerHealth(ctx)

	// Work on closing auctions.
	ticker := newAuctionCloseTicker(am.roundDuration, am.auctionClosingDurationBeforeRoundStart)
	go ticker.start()
	defer func() {
		ticker.done <- true
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case auctionClosingTime := <-ticker.c:
			log.Info("Auction closing", "closingTime", auctionClosingTime)
			if err := am.resolveAuctions(ctx); err != nil {
				log.Error("Could not resolve auction for round", "error", err)
			}
		}
	}
}

func (am *auctionMaster) resolveAuctions(ctx context.Context) error {
	upcomingRound := currentRound(am.initialRoundTimestamp, am.roundDuration) + 1
	// If we have no winner, then we can cancel the auction.
	// Auction master can also subscribe to sequencer feed and
	// close auction if sequencer is down.
	result := am.bidCache.topTwoBids()
	first := result.firstPlace
	second := result.secondPlace
	var tx *types.Transaction
	var err error
	hasSingleBid := first != nil && second == nil
	hasBothBids := first != nil && second != nil
	noBids := first == nil && second == nil

	// TODO: Retry a given number of times in case of flakey connection.
	switch {
	case hasBothBids:
		tx, err = am.auctionContract.ResolveAuction(
			am.txOpts,
			bindings.Bid{
				Bidder:    first.address,
				ChainId:   am.chainId,
				Round:     new(big.Int).SetUint64(upcomingRound),
				Amount:    first.amount,
				Signature: first.signature,
			},
			bindings.Bid{
				Bidder:    second.address,
				ChainId:   am.chainId,
				Round:     new(big.Int).SetUint64(upcomingRound),
				Amount:    second.amount,
				Signature: second.signature,
			},
		)
	case hasSingleBid:
		tx, err = am.auctionContract.ResolveSingleBidAuction(
			am.txOpts,
			bindings.Bid{
				Bidder:    first.address,
				ChainId:   am.chainId,
				Round:     new(big.Int).SetUint64(upcomingRound),
				Amount:    first.amount,
				Signature: first.signature,
			},
		)
	case noBids:
		// TODO: Cancel the upcoming auction.
		log.Info("No bids received for auction resolution")
		return nil
	}
	if err != nil {
		return err
	}
	receipt, err := bind.WaitMined(ctx, am.client, tx)
	if err != nil {
		return err
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return errors.New("deposit failed")
	}
	// Clear the bid cache.
	am.bidCache = newBidCache()
	return nil
}

// TODO: Implement. If sequencer is down for some time, cancel the upcoming auction by calling
// the cancel method on the smart contract.
func (am *auctionMaster) checkSequencerHealth(ctx context.Context) {

}

func currentRound(initialRoundTimestamp time.Time, roundDuration time.Duration) uint64 {
	return uint64(time.Since(initialRoundTimestamp) / roundDuration)
}

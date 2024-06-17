package timeboost

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/log"
	"github.com/offchainlabs/nitro/execution/gethexec/timeboost/bindings"
	"github.com/pkg/errors"
)

type sequencerConnection interface {
	SendExpressLaneTx(ctx context.Context, tx *types.Transaction) error
}

type auctionMasterConnection interface {
	SubmitBid(ctx context.Context, bid *bid) error
}

type bidderClient struct {
	chainId               uint64
	name                  string
	signatureDomain       uint16
	txOpts                *bind.TransactOpts
	client                simulated.Client
	privKey               *ecdsa.PrivateKey
	auctionContract       *bindings.ExpressLaneAuction
	sequencer             sequencerConnection
	auctionMaster         auctionMasterConnection
	initialRoundTimestamp time.Time
	roundDuration         time.Duration
}

// TODO: Provide a safer option.
type wallet struct {
	txOpts  *bind.TransactOpts
	privKey *ecdsa.PrivateKey
}

func newBidderClient(
	ctx context.Context,
	name string,
	wallet *wallet,
	client simulated.Client,
	auctionContractAddress common.Address,
	sequencer sequencerConnection,
	auctionMaster auctionMasterConnection,
) (*bidderClient, error) {
	chainId, err := client.ChainID(ctx)
	if err != nil {
		return nil, err
	}
	auctionContract, err := bindings.NewExpressLaneAuction(auctionContractAddress, client)
	if err != nil {
		return nil, err
	}
	sigDomain, err := auctionContract.BidSignatureDomainValue(&bind.CallOpts{})
	if err != nil {
		return nil, err
	}
	initialRoundTimestamp, err := auctionContract.InitialRoundTimestamp(&bind.CallOpts{})
	if err != nil {
		return nil, err
	}
	roundDurationSeconds, err := auctionContract.RoundDurationSeconds(&bind.CallOpts{})
	if err != nil {
		return nil, err
	}
	return &bidderClient{
		chainId:               chainId.Uint64(),
		name:                  name,
		signatureDomain:       sigDomain,
		client:                client,
		txOpts:                wallet.txOpts,
		privKey:               wallet.privKey,
		auctionContract:       auctionContract,
		sequencer:             sequencer,
		auctionMaster:         auctionMaster,
		initialRoundTimestamp: time.Unix(initialRoundTimestamp.Int64(), 0),
		roundDuration:         time.Duration(roundDurationSeconds) * time.Second,
	}, nil
}

func (bd *bidderClient) Start(ctx context.Context) {
	// Monitor for newly assigned express lane controllers, and if the client's address
	// is the controller in order to send express lane txs.
	go bd.monitorAuctionResolutions(ctx)
	// Monitor for auction closures by the auction master.
	go bd.monitorAuctionCancelations(ctx)
	// Monitor for express lane control delegations to take over if needed.
	go bd.monitorExpressLaneDelegations(ctx)
}

func (bd *bidderClient) monitorAuctionResolutions(ctx context.Context) {
	winningBidders := []common.Address{bd.txOpts.From}
	latestBlock, err := bd.client.HeaderByNumber(ctx, nil)
	if err != nil {
		panic(err)
	}
	fromBlock := latestBlock.Number.Uint64()
	ticker := time.NewTicker(time.Millisecond * 250)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			latestBlock, err := bd.client.HeaderByNumber(ctx, nil)
			if err != nil {
				log.Error("Could not get latest header", "err", err)
				continue
			}
			toBlock := latestBlock.Number.Uint64()
			if fromBlock == toBlock {
				continue
			}
			filterOpts := &bind.FilterOpts{
				Context: ctx,
				Start:   fromBlock,
				End:     &toBlock,
			}
			it, err := bd.auctionContract.FilterAuctionResolved(filterOpts, winningBidders, nil)
			if err != nil {
				log.Error("Could not filter auction resolutions", "error", err)
				continue
			}
			for it.Next() {
				upcomingRound := CurrentRound(bd.initialRoundTimestamp, bd.roundDuration) + 1
				ev := it.Event
				if ev.WinnerRound.Uint64() == upcomingRound {
					// TODO: Log the time to next round.
					log.Info(
						"WON the express lane auction for next round - can send fast lane txs to sequencer",
						"winner", ev.WinningBidder,
						"upcomingRound", upcomingRound,
						"firstPlaceBidAmount", fmt.Sprintf("%#x", ev.WinningBidAmount),
						"secondPlaceBidAmount", fmt.Sprintf("%#x", ev.WinningBidAmount),
					)
				}
			}
		}
	}
}

func (bd *bidderClient) monitorAuctionCancelations(ctx context.Context) {
	// TODO: Implement.
}

func (bd *bidderClient) monitorExpressLaneDelegations(ctx context.Context) {
	delegatedTo := []common.Address{bd.txOpts.From}
	latestBlock, err := bd.client.HeaderByNumber(ctx, nil)
	if err != nil {
		panic(err)
	}
	fromBlock := latestBlock.Number.Uint64()
	ticker := time.NewTicker(time.Millisecond * 250)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			latestBlock, err := bd.client.HeaderByNumber(ctx, nil)
			if err != nil {
				log.Error("Could not get latest header", "err", err)
				continue
			}
			toBlock := latestBlock.Number.Uint64()
			if fromBlock == toBlock {
				continue
			}
			filterOpts := &bind.FilterOpts{
				Context: ctx,
				Start:   fromBlock,
				End:     &toBlock,
			}
			it, err := bd.auctionContract.FilterExpressLaneControlDelegated(filterOpts, nil, delegatedTo)
			if err != nil {
				log.Error("Could not filter auction resolutions", "error", err)
				continue
			}
			for it.Next() {
				upcomingRound := CurrentRound(bd.initialRoundTimestamp, bd.roundDuration) + 1
				ev := it.Event
				// TODO: Log the time to next round.
				log.Info(
					"Received express lane delegation for next round -  can send fast lane txs to sequencer",
					"delegatedFrom", ev.From,
					"upcomingRound", upcomingRound,
				)
			}
		}
	}
}

func (bd *bidderClient) sendExpressLaneTx(ctx context.Context, tx *types.Transaction) error {
	return bd.sequencer.SendExpressLaneTx(ctx, tx)
}

func (bd *bidderClient) deposit(ctx context.Context, amount *big.Int) error {
	tx, err := bd.auctionContract.SubmitDeposit(bd.txOpts, amount)
	if err != nil {
		return err
	}
	receipt, err := bind.WaitMined(ctx, bd.client, tx)
	if err != nil {
		return err
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return errors.New("deposit failed")
	}
	return nil
}

func (bd *bidderClient) bid(ctx context.Context, amount *big.Int) (*bid, error) {
	newBid := &bid{
		chainId: bd.chainId,
		address: bd.txOpts.From,
		round:   CurrentRound(bd.initialRoundTimestamp, bd.roundDuration) + 1,
		amount:  amount,
	}
	packedBidBytes, err := encodeBidValues(
		bd.signatureDomain, new(big.Int).SetUint64(newBid.chainId), new(big.Int).SetUint64(newBid.round), amount,
	)
	if err != nil {
		return nil, err
	}
	sig, prefixed := sign(packedBidBytes, bd.privKey)
	newBid.signature = sig
	_ = prefixed
	if err = bd.auctionMaster.SubmitBid(ctx, newBid); err != nil {
		return nil, err
	}
	return newBid, nil
}

func sign(message []byte, key *ecdsa.PrivateKey) ([]byte, []byte) {
	hash := crypto.Keccak256(message)
	prefixed := crypto.Keccak256([]byte("\x19Ethereum Signed Message:\n32"), hash)
	sig, err := secp256k1.Sign(prefixed, math.PaddedBigBytes(key.D, 32))
	if err != nil {
		panic(err)
	}
	return sig, prefixed
}

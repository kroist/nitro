package gethexec

import (
	"container/heap"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
)

const (
	defaultMaxBoostFactor = 500 * time.Millisecond
)

type timeBoostHeap[T boostableTx] struct {
	sync.RWMutex
	prioQueue timeBoostableTxs[T]
	gFactor   time.Duration
}

func newTimeBoostHeap[T boostableTx](opts ...timeBoostOpt[T]) *timeBoostHeap[T] {
	prioQueue := make(timeBoostableTxs[T], 0)
	heap.Init(&prioQueue)
	srv := &timeBoostHeap[T]{
		gFactor:   defaultMaxBoostFactor,
		prioQueue: prioQueue,
	}
	for _, o := range opts {
		o(srv)
	}
	return srv
}

type timeBoostOpt[T boostableTx] func(*timeBoostHeap[T])

// Sets the "G" parameter for time boost, in milliseconds.
func withTimeboostParameter[T boostableTx](gFactor uint64) timeBoostOpt[T] {
	return func(s *timeBoostHeap[T]) {
		s.gFactor = time.Millisecond * s.gFactor
	}
}

func (tb *timeBoostHeap[T]) PushAll(txs []T) {
	tb.Lock()
	defer tb.Unlock()
	for _, tx := range txs {
		heap.Push(&tb.prioQueue, tx)
	}
}

func (tb *timeBoostHeap[T]) PopAll() []T {
	tb.Lock()
	defer tb.Unlock()
	txs := make([]T, 0, tb.prioQueue.Len())
	for tb.prioQueue.Len() > 0 {
		txs = append(txs, heap.Pop(&tb.prioQueue).(T))
	}
	return txs
}

// A boostable tx type that contains a bid and a timestamp.
type boostableTx interface {
	id() string
	bid() uint64
	timestamp() time.Time
	innerTx() *types.Transaction
}

// Defines a type that implements the heap.Interface interface from
// the standard library.
type timeBoostableTxs[T boostableTx] []T

func (tb timeBoostableTxs[T]) Len() int      { return len(tb) }
func (tb timeBoostableTxs[T]) Swap(i, j int) { tb[i], tb[j] = tb[j], tb[i] }

// We want to implement a priority queue using a max heap.
func (tb timeBoostableTxs[T]) Less(i, j int) bool {
	if tb[i].bid() == tb[j].bid() {
		// Ties are broken by earliest timestamp.
		return tb[i].timestamp().Before(tb[j].timestamp())
	}
	return tb[i].bid() > tb[j].bid()
}

// Push and Pop implement the required methods for the heap interface from the standard library.
func (tb *timeBoostableTxs[T]) Push(item any) {
	tx := item.(T)
	*tb = append(*tb, tx)
}

func (tb *timeBoostableTxs[T]) Pop() any {
	old := *tb
	n := len(old)
	item := old[n-1]
	var zero T
	old[n-1] = zero // avoid memory leak
	*tb = old[0 : n-1]
	return item
}

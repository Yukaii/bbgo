package depth

import (
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

type SnapshotFetcher func() (snapshot types.SliceOrderBook, finalUpdateID int64, err error)

type Update struct {
	FirstUpdateID, FinalUpdateID int64

	// Object is the update object
	Object types.SliceOrderBook
}

//go:generate callbackgen -type Buffer
type Buffer struct {
	buffer []Update

	finalUpdateID int64
	fetcher       SnapshotFetcher
	snapshot      *types.SliceOrderBook

	resetCallbacks []func()
	readyCallbacks []func(snapshot types.SliceOrderBook, updates []Update)
	pushCallbacks  []func(update Update)

	resetC chan struct{}
	mu     sync.Mutex
	once   util.Reonce

	// updateTimeout the timeout duration when not receiving update messages
	updateTimeout time.Duration

	// bufferingPeriod is used to buffer the update message before we get the full depth
	bufferingPeriod time.Duration
}

func NewBuffer(fetcher SnapshotFetcher) *Buffer {
	return &Buffer{
		fetcher: fetcher,
		resetC:  make(chan struct{}, 1),
	}
}

func (b *Buffer) SetUpdateTimeout(d time.Duration) {
	b.updateTimeout = d
}

func (b *Buffer) SetBufferingPeriod(d time.Duration) {
	b.bufferingPeriod = d
}

func (b *Buffer) resetSnapshot() {
	b.snapshot = nil
	b.finalUpdateID = 0
	b.EmitReset()
}

func (b *Buffer) emitReset() {
	select {
	case b.resetC <- struct{}{}:
	default:
	}
}

func (b *Buffer) Reset() {
	b.mu.Lock()
	b.resetSnapshot()
	b.emitReset()
	b.mu.Unlock()
}

// AddUpdate adds the update to the buffer or push the update to the subscriber
func (b *Buffer) AddUpdate(o types.SliceOrderBook, firstUpdateID int64, finalArgs ...int64) error {
	finalUpdateID := firstUpdateID
	if len(finalArgs) > 0 {
		finalUpdateID = finalArgs[0]
	}

	u := Update{
		FirstUpdateID: firstUpdateID,
		FinalUpdateID: finalUpdateID,
		Object:        o,
	}

	// we lock here because there might be 2+ calls to the AddUpdate method
	// we don't want to reset sync.Once 2 times here
	b.mu.Lock()
	select {
	case <-b.resetC:
		log.Warnf("received depth reset signal, resetting...")

		// if the once goroutine is still running, overwriting this once might cause "unlock of unlocked mutex" panic.
		b.once.Reset()
	default:
	}

	// if the snapshot is set to nil, we need to buffer the message
	if b.snapshot == nil {
		b.buffer = append(b.buffer, u)
		b.once.Do(func() {
			go b.tryFetch()
		})
		b.mu.Unlock()
		return nil
	}

	// if there is a missing update, we should reset the snapshot and re-fetch the snapshot
	if u.FirstUpdateID > b.finalUpdateID+1 {
		// emitReset will reset the once outside the mutex lock section
		b.buffer = []Update{u}
		b.resetSnapshot()
		b.emitReset()
		b.mu.Unlock()
		return fmt.Errorf("found missing update between finalUpdateID %d and firstUpdateID %d, diff: %d",
			b.finalUpdateID+1,
			u.FirstUpdateID,
			u.FirstUpdateID-b.finalUpdateID)
	}

	log.Debugf("depth update id %d -> %d", b.finalUpdateID, u.FinalUpdateID)
	b.finalUpdateID = u.FinalUpdateID
	b.EmitPush(u)
	b.mu.Unlock()
	return nil
}

func (b *Buffer) fetchAndPush() error {
	book, finalUpdateID, err := b.fetcher()
	if err != nil {
		return err
	}

	log.Debugf("fetched depth snapshot, final update id %d", finalUpdateID)

	b.mu.Lock()
	if len(b.buffer) > 0 {
		// the snapshot is too early
		if finalUpdateID < b.buffer[0].FirstUpdateID {
			b.resetSnapshot()
			b.emitReset()
			b.mu.Unlock()
			return fmt.Errorf("depth snapshot is too early, final update %d is < the first update id %d", finalUpdateID, b.buffer[0].FirstUpdateID)
		}
	}

	var pushUpdates []Update
	for _, u := range b.buffer {
		// skip old events
		if u.FirstUpdateID < finalUpdateID+1 {
			continue
		}

		if u.FirstUpdateID > finalUpdateID+1 {
			b.resetSnapshot()
			b.emitReset()
			b.mu.Unlock()
			return fmt.Errorf("there is a missing depth update, the update id %d > final update id %d + 1", u.FirstUpdateID, finalUpdateID)
		}

		pushUpdates = append(pushUpdates, u)

		// update the final update id to the correct final update id
		finalUpdateID = u.FinalUpdateID
	}

	// clean the buffer since we have filtered out the buffer we want
	b.buffer = nil

	// set the final update ID so that we will know if there is an update missing
	b.finalUpdateID = finalUpdateID

	// set the snapshot
	b.snapshot = &book

	b.mu.Unlock()
	b.EmitReady(book, pushUpdates)
	return nil
}

func (b *Buffer) tryFetch() {
	for {
		if b.bufferingPeriod > 0 {
			<-time.After(b.bufferingPeriod)
		}

		err := b.fetchAndPush()
		if err != nil {
			log.WithError(err).Errorf("snapshot fetch failed")
			continue
		}
		break
	}
}

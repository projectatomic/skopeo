package dht

import (
	"sync"
	"time"

	"github.com/anacrolix/dht/krpc"
)

// Transaction keeps track of a message exchange between nodes, such as a
// query message and a response message.
type Transaction struct {
	remoteAddr       Addr
	t                string
	onResponse       func(krpc.Msg)
	onTimeout        func()
	onSendError      func(error)
	querySender      func() error
	queryResendDelay func() time.Duration

	mu          sync.Mutex
	gotResponse bool
	timer       *time.Timer
	retries     int
	lastSend    time.Time
}

func (t *Transaction) handleResponse(m krpc.Msg) {
	t.mu.Lock()
	t.gotResponse = true
	t.mu.Unlock()
	t.onResponse(m)
}

func (t *Transaction) key() transactionKey {
	return transactionKey{
		t.remoteAddr.String(),
		t.t,
	}
}

func (t *Transaction) startResendTimer() {
	t.timer = time.AfterFunc(0, t.resendCallback)
}

func (t *Transaction) resendCallback() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.gotResponse {
		return
	}
	if t.retries == 3 {
		go t.onTimeout()
		return
	}
	t.retries++
	if err := t.sendQuery(); err != nil {
		go t.onSendError(err)
		return
	}
	if t.timer.Reset(t.queryResendDelay()) {
		panic("timer should have fired to get here")
	}
}

func (t *Transaction) sendQuery() error {
	if err := t.querySender(); err != nil {
		return err
	}
	t.lastSend = time.Now()
	return nil
}

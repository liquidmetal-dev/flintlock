package grpc_test

import (
	"context"
	"sync"

	"google.golang.org/grpc/metadata"
)

// recvItem is one scripted result for fakeBidiStream.Recv.
type recvItem[Req any] struct {
	msg *Req
	err error
}

// fakeBidiStream is a minimal grpc.BidiStreamingServer[Req, Resp] test double.
// Recv results are fed via pushRecv/pushErr (buffered, so a test can script a
// whole exchange up front for a synchronous call, or push incrementally from
// another goroutine to control timing against a handler running in the
// background). Sent responses are captured for assertions via Sent.
type fakeBidiStream[Req any, Resp any] struct {
	ctx    context.Context
	recvCh chan recvItem[Req]

	mu   sync.Mutex
	sent []*Resp
}

func newFakeBidiStream[Req any, Resp any]() *fakeBidiStream[Req, Resp] {
	return &fakeBidiStream[Req, Resp]{
		ctx:    context.Background(),
		recvCh: make(chan recvItem[Req], 16),
	}
}

func (f *fakeBidiStream[Req, Resp]) pushRecv(msg *Req) { f.recvCh <- recvItem[Req]{msg: msg} }
func (f *fakeBidiStream[Req, Resp]) pushErr(err error) { f.recvCh <- recvItem[Req]{err: err} }

func (f *fakeBidiStream[Req, Resp]) Recv() (*Req, error) {
	item := <-f.recvCh

	return item.msg, item.err
}

func (f *fakeBidiStream[Req, Resp]) Send(m *Resp) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.sent = append(f.sent, m)

	return nil
}

// Sent returns a snapshot of every response sent so far.
func (f *fakeBidiStream[Req, Resp]) Sent() []*Resp {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]*Resp, len(f.sent))
	copy(out, f.sent)

	return out
}

func (f *fakeBidiStream[Req, Resp]) Context() context.Context     { return f.ctx }
func (f *fakeBidiStream[Req, Resp]) SetHeader(metadata.MD) error  { return nil }
func (f *fakeBidiStream[Req, Resp]) SendHeader(metadata.MD) error { return nil }
func (f *fakeBidiStream[Req, Resp]) SetTrailer(metadata.MD)       {}
func (f *fakeBidiStream[Req, Resp]) SendMsg(m any) error          { return nil }
func (f *fakeBidiStream[Req, Resp]) RecvMsg(m any) error          { return nil }

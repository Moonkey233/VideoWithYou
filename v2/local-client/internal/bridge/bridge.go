package bridge

import "context"

type Host interface {
	Start(ctx context.Context)
	Send(msg any) error
	Incoming() <-chan []byte
}

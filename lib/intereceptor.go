package lib

import (
	"github.com/Jeffail/benthos/lib/types"
)

type InterceptingResponse struct {
	types.Response
	Value types.Message
}

type InterceptingOutput struct {
	types.Output
}

func (out *InterceptingOutput) adapt(src <-chan types.Transaction, dst chan<- types.Transaction) {
	for tx := range src {
		msg := tx.Payload
		rspCh := tx.ResponseChan
		capturedRspCh := make(chan types.Response, cap(rspCh))
		tx.ResponseChan = capturedRspCh
		dst <- tx
		rsp := <-capturedRspCh
		rspCh <- &InterceptingResponse{Response: rsp, Value: msg}
	}
}

func (out *InterceptingOutput) Consume(txs <-chan types.Transaction) error {
	adaptedTxs := make(chan types.Transaction, cap(txs))
	go out.adapt(txs, adaptedTxs)
	return out.Output.Consume(adaptedTxs)
}

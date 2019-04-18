package lib

import (
	"context"
	"encoding/json"

	"github.com/Jeffail/benthos/lib/message"
	"github.com/Jeffail/benthos/lib/types"
)

type Handler struct {
	Transactions chan types.Transaction
}

func (h *Handler) Handle(ctx context.Context, b json.RawMessage) (json.RawMessage, error) {
	msg := message.New([][]byte{b})
	respCh := make(chan types.Response, 1)
	h.Transactions <- types.Transaction{
		Payload:      msg,
		ResponseChan: respCh,
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		var fullResp *InterceptingResponse
		var ok bool
		if fullResp, ok = resp.(*InterceptingResponse); !ok {
			return nil, resp.Error()
		}
		return fullResp.Value.Get(0).Get(), fullResp.Error()
	}
}

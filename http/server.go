package jsonrpchttp

import (
	"bytes"
	//"context"
	"github.com/pkg/errors"
	"github.com/superisaac/jsonrpc"
	"net/http"
)

type Server struct {
	dispatcher *Dispatcher
}

func NewServer(dispatcher *Dispatcher) *Server {
	if dispatcher == nil {
		dispatcher = NewDispatcher()
	}
	return &Server{
		dispatcher: dispatcher,
	}
}

func (self *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// only support POST
	if r.Method != "POST" {
		jsonrpc.ErrorResponse(w, r, errors.New("method not allowed"), 405, "Method not allowed")
		return
	}

	// parsing http body
	var buffer bytes.Buffer
	_, err := buffer.ReadFrom(r.Body)
	if err != nil {
		jsonrpc.ErrorResponse(w, r, err, 400, "Bad request")
		return
	}

	msg, err := jsonrpc.ParseBytes(buffer.Bytes())
	if err != nil {
		jsonrpc.ErrorResponse(w, r, err, 400, "Bad jsonrpc request")
		return
	}

	if !msg.IsRequestOrNotify() {
		jsonrpc.ErrorResponse(w, r, err, 400, "Bad request, must be request or notify")
		return
	}

	resmsg, err := self.dispatcher.handleMessage(r.Context(), msg)
	if err != nil {
		msg.Log().Warnf("err.handleMessage %s", err)
		w.WriteHeader(500)
		w.Write([]byte("internal server error"))
		return
	}
	if msg.IsRequest() {
		if resmsg == nil {
			msg.Log().Panicf("resmsg is nil")
		}
		traceId := resmsg.TraceId()
		resmsg.SetTraceId("")

		data, err1 := jsonrpc.MessageBytes(resmsg)
		if err1 != nil {
			resmsg.Log().Warnf("error marshaling msg %s", err1)
			errmsg := jsonrpc.ErrInternalError.ToMessageFromId(msg.MustId(), msg.TraceId())
			data, _ = jsonrpc.MessageBytes(errmsg)
		}
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")

		if traceId != "" {
			w.Header().Set("X-Trace-Id", traceId)
		}
		w.Write(data)
	} else {
		w.WriteHeader(200)
		w.Write([]byte(""))
	}
} // Server.ServeHTTP
package jsonzhttp

import (
	"bufio"
	"context"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"io"
	"net/http"
)

type H2Handler struct {
	Actor     *Actor
	serverCtx context.Context
	// options
	SpawnGoroutine bool
}

type H2Session struct {
	server      *H2Handler
	scanner     *bufio.Scanner
	writer      io.Writer
	flusher     http.Flusher
	httpRequest *http.Request
	rootCtx     context.Context
	done        chan error
	sendChannel chan jsonz.Message
	streamId    string
}

func NewH2Handler(serverCtx context.Context, actor *Actor) *H2Handler {
	if actor == nil {
		actor = NewActor()
	}
	return &H2Handler{
		serverCtx: serverCtx,
		Actor:     actor,
	}
}

func (self *H2Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !r.ProtoAtLeast(2, 0) {
		//return fmt.Errorf("HTTP2 not supported")
		w.WriteHeader(400)
		w.Write([]byte("http2 not supported"))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(400)
		w.Write([]byte("http2 not supported"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{\"method\":\"hello\",\"params\":[]}\n"))
	flusher.Flush()

	scanner := bufio.NewScanner(r.Body)
	scanner.Split(bufio.ScanLines)
	defer func() {
		r.Body.Close()
		self.Actor.HandleClose(r)
	}()
	session := &H2Session{
		server:      self,
		rootCtx:     r.Context(),
		httpRequest: r,
		writer:      w,
		flusher:     flusher,
		scanner:     scanner,
		done:        make(chan error, 10),
		sendChannel: make(chan jsonz.Message, 100),
		streamId:    jsonz.NewUuid(),
	}
	session.wait()
}

// websocket session
func (self *H2Session) wait() {
	connCtx, cancel := context.WithCancel(self.rootCtx)
	defer cancel()

	serverCtx, cancelServer := context.WithCancel(self.server.serverCtx)
	defer cancelServer()

	go self.sendLoop()
	go self.recvLoop()

	for {
		select {
		case <-connCtx.Done():
			return
		case <-serverCtx.Done():
			return
		case err, ok := <-self.done:
			if ok && err != nil {
				log.Warnf("websocket error %s", err)
			}
			return
		}
	}
}

func (self *H2Session) recvLoop() {
	for self.scanner.Scan() {
		data := self.scanner.Bytes()
		if self.server.SpawnGoroutine {
			go self.msgBytesReceived(data)
		} else {
			self.msgBytesReceived(data)
		}
	}
	// end of scanning
	self.done <- nil
	return
}

func (self *H2Session) msgBytesReceived(msgBytes []byte) {
	msg, err := jsonz.ParseBytes(msgBytes)
	if err != nil {
		log.Warnf("bad jsonrpc message %s", msgBytes)
		self.done <- errors.New("bad jsonrpc message")
		return
	}

	req := NewRPCRequest(
		self.rootCtx,
		msg,
		TransportHTTP2,
		self.httpRequest,
		self)
	req.streamId = self.streamId

	resmsg, err := self.server.Actor.Feed(req)
	if err != nil {
		self.done <- errors.Wrap(err, "actor.Feed")
		return
	}
	if resmsg != nil {
		if resmsg.IsResultOrError() {
			self.sendChannel <- resmsg
		} else {
			self.Send(resmsg)
		}
	}
}

func (self *H2Session) Send(msg jsonz.Message) {
	self.sendChannel <- msg
}

func (self *H2Session) sendLoop() {
	ctx, cancel := context.WithCancel(self.rootCtx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-self.sendChannel:
			if !ok {
				return
			}
			if self.scanner == nil {
				return
			}
			marshaled, err := jsonz.MessageBytes(msg)
			if err != nil {
				log.Warnf("marshal msg error %s", err)
				return
			}

			marshaled = append(marshaled, []byte("\n")...)
			if _, err := self.writer.Write(marshaled); err != nil {
				log.Warnf("h2 writedata warning message %s\n", err)
				return
			}
			self.flusher.Flush()
		}
	}
}

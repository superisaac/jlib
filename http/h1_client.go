package jsonzhttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"github.com/pkg/errors"
	"github.com/superisaac/jsonz"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

type H1Client struct {
	serverUrl  string
	httpClient *http.Client

	connectOnce sync.Once

	clientTLS *tls.Config
}

func NewH1Client(serverUrl string) *H1Client {
	return &H1Client{serverUrl: serverUrl}
}

func (self *H1Client) connect() {
	self.connectOnce.Do(func() {
		tr := &http.Transport{
			MaxIdleConns:        30,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		}
		if self.clientTLS != nil {
			tr.TLSClientConfig = self.clientTLS
		}
		self.httpClient = &http.Client{
			Transport: tr,
			Timeout:   5 * time.Second,
		}
	})
}

func (self *H1Client) SetClientTLSConfig(cfg *tls.Config) {
	self.clientTLS = cfg
}

func (self *H1Client) UnwrapCall(rootCtx context.Context, reqmsg *jsonz.RequestMessage, output interface{}, headers ...http.Header) error {
	resmsg, err := self.Call(rootCtx, reqmsg, headers...)
	if err != nil {
		return err
	}
	if resmsg.IsResult() {
		err := jsonz.DecodeInterface(resmsg.MustResult(), output)
		if err != nil {
			return err
		}
		return nil
	} else {
		return resmsg.MustError()
	}
}

func (self *H1Client) Call(rootCtx context.Context, reqmsg *jsonz.RequestMessage, headers ...http.Header) (jsonz.Message, error) {
	self.connect()

	traceId := reqmsg.TraceId()

	reqmsg.SetTraceId("")

	marshaled, err := jsonz.MessageBytes(reqmsg)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(marshaled)

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", self.serverUrl, reader)
	if err != nil {
		return nil, errors.Wrap(err, "http.NewRequestWithContext")
	}
	if traceId != "" {
		req.Header.Add("X-Trace-Id", traceId)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	for _, extheader := range headers {
		for k, vs := range extheader {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}

	resp, err := self.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "http Do")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		abnResp := &WrappedResponse{
			Response: resp,
		}
		return nil, errors.Wrap(abnResp, "abnormal response")
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "ioutil.ReadAll")
	}
	respmsg, err := jsonz.ParseBytes(respBody)
	if err != nil {
		return nil, err
	}
	respmsg.SetTraceId(traceId)
	return respmsg, nil
}

func (self *H1Client) Send(rootCtx context.Context, msg jsonz.Message, headers ...http.Header) error {
	self.connect()

	traceId := msg.TraceId()
	msg.SetTraceId("")

	marshaled, err := jsonz.MessageBytes(msg)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(marshaled)

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", self.serverUrl, reader)
	if err != nil {
		return errors.Wrap(err, "http.NewRequestWithContext")
	}
	if traceId != "" {
		req.Header.Add("X-Trace-Id", traceId)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	for _, extheader := range headers {
		for k, vs := range extheader {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}

	resp, err := self.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "http Do")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		abnResp := &WrappedResponse{
			Response: resp,
		}
		return errors.Wrap(abnResp, "abnormal response")
	}
	return nil
}
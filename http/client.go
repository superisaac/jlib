package jsonrpchttp

import (
	"fmt"
	"io/ioutil"

	"bytes"
	"context"
	"github.com/pkg/errors"
	"github.com/superisaac/jsonrpc"
	"net/http"
	"time"
)

// errors
type AbnormalResponse struct {
	Response *http.Response
}

func (self AbnormalResponse) Error() string {
	return fmt.Sprintf("Abnormal response %d", self.Response.StatusCode)
}

type Client struct {
	serverUrl  string
	httpClient *http.Client
}

func NewClient(serverUrl string) *Client {
	return &Client{serverUrl: serverUrl}
}

func (self *Client) connect() {
	if self.httpClient == nil {
		tr := &http.Transport{
			MaxIdleConns:        30,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		}
		self.httpClient = &http.Client{
			Transport: tr,
			Timeout:   5 * time.Second,
		}
	}
}

func (self *Client) Call(rootCtx context.Context, reqmsg *jsonrpc.RequestMessage) (jsonrpc.IMessage, error) {
	self.connect()

	traceId := reqmsg.TraceId()

	reqmsg.SetTraceId("")

	marshaled, err := jsonrpc.MessageBytes(reqmsg)
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
	req.Header.Add("X-Trace-Id", traceId)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// if self.Headers != nil {
	//         for k, v := range self.Headers {
	//                 req.Header.Set(k, v)
	//         }
	// }

	resp, err := self.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "http Do")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		abnResp := &AbnormalResponse{
			Response: resp,
		}
		return nil, errors.Wrap(abnResp, "abnormal response")
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "ioutil.ReadAll")
	}
	respmsg, err := jsonrpc.ParseBytes(respBody)
	if err != nil {
		return nil, err
	}
	respmsg.SetTraceId(traceId)
	return respmsg, nil
}

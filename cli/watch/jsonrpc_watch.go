package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"github.com/superisaac/jsonz/http"
	"net/http"
	"os"
	"time"
)

func main() {
	cliFlags := flag.NewFlagSet("jsonrpc-watch", flag.ExitOnError)
	pServerUrl := cliFlags.String("c", "", "jsonrpc server url, wss? prefixed, can be in env JSONRPC_CONNECT, default is ws://127.0.0.1:9990")
	pRetry := cliFlags.Int("retry", 1, "retry times")
	var headerFlags jsonzhttp.HeaderFlags
	cliFlags.Var(&headerFlags, "header", "attached http headers")
	cliFlags.Parse(os.Args[1:])

	log.SetOutput(os.Stderr)

	// parse server url
	serverUrl := *pServerUrl
	if serverUrl == "" {
		serverUrl = os.Getenv("JSONRPC_CONNECT")
	}

	if serverUrl == "" {
		serverUrl = "ws://127.0.0.1:9990"
	}

	// parse http headers
	headers := []http.Header{}
	h, err := headerFlags.Parse()
	if err != nil {
		log.Fatalf("err parse header flags %s", err)
		os.Exit(1)
	}
	if len(h) > 0 {
		headers = append(headers, h)
	}

	// parse method and params
	var method string
	var params []interface{}
	if cliFlags.NArg() >= 1 {
		args := cliFlags.Args()
		method = args[0]
		clParams := args[1:len(args)]

		p1, err := jsonz.GuessJsonArray(clParams)
		if err != nil {
			log.Fatalf("params error: %s", err)
			os.Exit(1)
		}
		params = p1
	}

	// jsonz client
	c, err := jsonzhttp.NewClient(serverUrl)
	if err != nil {
		log.Fatalf("fail to find jsonrpc client: %s", err)
		os.Exit(1)
	}
	sc, ok := c.(*jsonzhttp.WSClient)
	if !ok {
		log.Panicf("websocket client required, but found %s", c)
		os.Exit(1)
	}

	sc.OnMessage(func(msg jsonz.Message) {
		repr, err := jsonz.EncodePretty(msg)
		if err != nil {
			//panic(err)
			log.Panicf("on message %s", err)
		}
		fmt.Println(repr)
	})

	retrytimes := 0

	for {
		if err := watchStreaming(sc, method, params, headers); err != nil {
			if errors.Is(err, jsonzhttp.TransportConnectFailed) {
				retrytimes++
				log.Infof("connect refused %d times", retrytimes)
				if retrytimes >= *pRetry {
					break
				} else {
					time.Sleep(1 * time.Second)
					continue
				}
			} else {
				log.Errorf("watch error %s", err)
				break
				//panic(err)
			}
		} else {
			retrytimes = 0
		}
	}
}

func watchStreaming(sc *jsonzhttp.WSClient, method string, params []interface{}, headers []http.Header) error {
	ctx1, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sc.Connect(ctx1, headers...); err != nil {
		return err
	}

	if method != "" {
		reqId := jsonz.NewUuid()
		reqmsg := jsonz.NewRequestMessage(reqId, method, params)
		resmsg, err := sc.Call(ctx1, reqmsg, headers...)
		if err != nil {
			sc.Log().Panicf("rpc error: %s", err)
			os.Exit(1)
		}
		repr, err := jsonz.EncodePretty(resmsg)
		if err != nil {
			sc.Log().Panicf("encode pretty error %s", err)
		}
		fmt.Println(repr)
	}
	// wait loop
	cerr := sc.Wait()
	if cerr != nil {
		sc.Log().Infof("client closed on error %s", cerr)
	} else {
		sc.Log().Debug("client closed")
	}
	return nil
}

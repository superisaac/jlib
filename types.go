// jsonz is JSONRPC 2.0 libaray in golang
package jsonz

import (
	"github.com/bitly/go-simplejson"
	log "github.com/sirupsen/logrus"
)

type UID string

// message kinds
const (
	MKRequest = iota
	MKNotify
	MKResult
	MKError
)

// RPC error object
type RPCError struct {
	Code    int
	Message string
	Data    interface{}
}

// The abstract interface of JSONRPC message. refer to
// https://www.jsonrpc.org/specification
//
// data Message = Request id method params |
//                Notify method params |
//                Result id result |
//                Error id error={ code message data }
//
type Message interface {
	// Return's the judgement of message types
	IsRequest() bool
	IsNotify() bool
	IsRequestOrNotify() bool
	IsResult() bool
	IsError() bool
	IsResultOrError() bool

	// TraceId can be used to analyse the flow of whole message
	// transportation
	SetTraceId(traceId string)
	TraceId() string

	//
	GetJson() *simplejson.Json

	// MustXX are convenience methods to make code cleaner by
	// avoiding frequent type casting, Note that there will be
	// panics when used inproperly, add some IsXX type checking
	// beforehead to add guarantee.

	// MustId returns the Id field of a message, will panic when
	// message is a Notify
	MustId() interface{}

	// MustMethod returns the method of a message, will panic when
	// message is an Result or Error.
	MustMethod() string

	// MustParams returns the params of a message, will panic when
	// message is a Result or Error
	MustParams() []interface{}

	// MustResult returns the result field of a message, will
	// panic when the message is not a Result
	MustResult() interface{}

	// MustError returns the error field of a message, will panic
	// when the message is not an Error
	MustError() *RPCError

	// Clone the message with a new Id
	ReplaceId(interface{}) Message

	// Log returns a Logger object with message specific
	// infomations attached.
	Log() *log.Entry
}

// The base class of JSONRPC types
type BaseMessage struct {
	kind    int
	raw     *simplejson.Json
	traceId string
}

// Request message kind
type RequestMessage struct {
	BaseMessage
	Id            interface{}
	Method        string
	Params        []interface{}
	paramsAreList bool

	// request specific fields
}

// Notify message kind
type NotifyMessage struct {
	BaseMessage
	Method        string
	Params        []interface{}
	paramsAreList bool
}

// Result message kind
type ResultMessage struct {
	BaseMessage
	Id     interface{}
	Result interface{}
}

// Error message kind
type ErrorMessage struct {
	BaseMessage
	Id    interface{}
	Error *RPCError
}

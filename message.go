package tao

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/xu215740578/logger"
)

const (
	// HeartBeat is the default heart beat message number.
	HeartBeat = 0
)

// Handler takes the responsibility to handle incoming messages.
type Handler interface {
	Handle(context.Context, interface{})
}

// HandlerFunc serves as an adapter to allow the use of ordinary functions as handlers.
type HandlerFunc func(context.Context, WriteCloser)

// Handle calls f(ctx, c)
func (f HandlerFunc) Handle(ctx context.Context, c WriteCloser) {
	f(ctx, c)
}

// UnmarshalFunc unmarshals bytes into Message.
type UnmarshalFunc func([]byte) (Message, error)

//HeadHandlerFunc 解析消息体长度
type HeadHandlerFunc func([]byte) (uint, error)

//TypeHandlerFunc 解析出该类型消息头长度
type TypeHandlerFunc func(uint16) (uint, error)

// handlerUnmarshaler is a combination of unmarshal and handle functions for message.
type handlerUnmarshaler struct {
	typehandler TypeHandlerFunc
	handler     HandlerFunc
	unmarshaler UnmarshalFunc
	headhandler HeadHandlerFunc
}

var (
	buf *bytes.Buffer
	// messageRegistry is the registry of all
	// message-related unmarshal and handle functions.
	messageRegistry map[uint16]handlerUnmarshaler
)

func init() {
	messageRegistry = map[uint16]handlerUnmarshaler{}
	buf = new(bytes.Buffer)
}

// Register registers the unmarshal and handle functions for msgType.
// If no unmarshal function provided, the message will not be parsed.
// If no handler function provided, the message will not be handled unless you
// set a default one by calling SetOnMessageCallback.
// If Register being called twice on one msgType, it will panics.
func Register(msgType uint16, unmarshaler func([]byte) (Message, error), handler func(context.Context, WriteCloser), typehandler func(uint16) (uint, error), headhandler func([]byte) (uint, error)) {
	if _, ok := messageRegistry[msgType]; ok {
		panic(fmt.Sprintf("trying to register message %d twice", msgType))
	}

	messageRegistry[msgType] = handlerUnmarshaler{
		typehandler: typehandler,
		unmarshaler: unmarshaler,
		handler:     HandlerFunc(handler),
		headhandler: headhandler,
	}
}

// GetUnmarshalFunc returns the corresponding unmarshal function for msgType.
func GetUnmarshalFunc(msgType uint16) UnmarshalFunc {
	entry, ok := messageRegistry[msgType]
	if !ok {
		return nil
	}
	return entry.unmarshaler
}

// GetTypeHandlerFunc returns the corresponding type handler function for msgType.
func GetTypeHandlerFunc(msgType uint16) TypeHandlerFunc {
	entry, ok := messageRegistry[msgType]
	if !ok {
		return nil
	}
	return entry.typehandler
}

// GetHeadHandlerFunc returns the corresponding head handler function for msgType.
func GetHeadHandlerFunc(msgType uint16) HeadHandlerFunc {
	entry, ok := messageRegistry[msgType]
	if !ok {
		return nil
	}
	return entry.headhandler
}

// GetHandlerFunc returns the corresponding handler function for msgType.
func GetHandlerFunc(msgType uint16) HandlerFunc {
	entry, ok := messageRegistry[msgType]
	if !ok {
		return nil
	}
	return entry.handler
}

// Message represents the structured data that can be handled.
type Message interface {
	MessageNumber() uint16
	Serialize() ([]byte, error)
}

// Codec is the interface for message coder and decoder.
// Application programmer can define a custom codec themselves.
type Codec interface {
	Decode(net.Conn) (Message, error)
	Encode(Message) ([]byte, error)
}

// TypeLengthValueCodec defines a special codec.
// Format: type-length-value |4 bytes|4 bytes|n bytes <= 8M|
type TypeLengthValueCodec struct{}

// Decode decodes the bytes data into Message
func (codec TypeLengthValueCodec) Decode(raw net.Conn) (Message, error) {
	byteChan := make(chan []byte)
	errorChan := make(chan error)

	go func(bc chan []byte, ec chan error) {
		typeData := make([]byte, MessageTypeBytes)
		_, err := io.ReadFull(raw, typeData)
		if err != nil {
			ec <- err
			close(bc)
			close(ec)
			logger.Debugln("go-routine read message type exited", err)
			return
		}
		bc <- typeData
	}(byteChan, errorChan)

	var typeBytes []byte

	select {
	case err := <-errorChan:
		return nil, err

	case typeBytes = <-byteChan:
		if typeBytes == nil {
			return nil, ErrBadData
		}
		typeBuf := bytes.NewReader(typeBytes)
		var msgType uint16
		if err := binary.Read(typeBuf, binary.BigEndian, &msgType); err != nil {
			return nil, err
		}

		typehandler := GetTypeHandlerFunc(msgType)
		headhandler := GetHeadHandlerFunc(msgType)
		unmarshaler := GetUnmarshalFunc(msgType)
		if typehandler == nil || headhandler == nil || unmarshaler == nil {
			return nil, ErrUndefined(msgType)
		}

		msgHeadLen, errtype := typehandler(msgType)
		if errtype != nil {
			return nil, errtype
		}

		lengthBytes := make([]byte, msgHeadLen)
		_, err := io.ReadFull(raw, lengthBytes)
		if err != nil {
			return nil, err
		}

		headBytes := append(typeBytes, lengthBytes...)
		var msgLen uint
		msgLen, err = headhandler(headBytes)
		if err != nil {
			return nil, err
		}

		if msgLen > MessageMaxBytes {
			logger.Errorf("message(type %d) has bytes(%d) beyond max %d\n", msgType, msgLen, MessageMaxBytes)
			return nil, ErrBadData
		}

		// read application data
		msgBytes := make([]byte, msgLen)
		_, err = io.ReadFull(raw, msgBytes)
		if err != nil {
			return nil, err
		}

		// deserialize message from bytes
		return unmarshaler(append(headBytes, msgBytes...))
	}
}

// Encode encodes the message into bytes data.
func (codec TypeLengthValueCodec) Encode(msg Message) ([]byte, error) {
	data, err := msg.Serialize()
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	buf.Write(data)
	packet := buf.Bytes()
	return packet, nil
}

// ContextKey is the key type for putting context-related data.
type contextKey string

// Context keys for messge, server and net ID.
const (
	messageCtx contextKey = "message"
	serverCtx  contextKey = "server"
	netIDCtx   contextKey = "netid"
)

// NewContextWithMessage returns a new Context that carries message.
func NewContextWithMessage(ctx context.Context, msg Message) context.Context {
	return context.WithValue(ctx, messageCtx, msg)
}

// MessageFromContext extracts a message from a Context.
func MessageFromContext(ctx context.Context) Message {
	return ctx.Value(messageCtx).(Message)
}

// NewContextWithNetID returns a new Context that carries net ID.
func NewContextWithNetID(ctx context.Context, netID int64) context.Context {
	return context.WithValue(ctx, netIDCtx, netID)
}

// NetIDFromContext returns a net ID from a Context.
func NetIDFromContext(ctx context.Context) int64 {
	return ctx.Value(netIDCtx).(int64)
}

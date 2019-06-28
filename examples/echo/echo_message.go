package echo

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/xu215740578/holmes"
	"github.com/xu215740578/tao"
)

// UsMessage defines the 'US' type message.
type UsMessage struct {
	Header  UsMessageHeader
	Content []byte
}

type UsMessageHeader struct {
	Type     uint16
	Version  uint8
	Session  [12]byte
	Encrypt  uint8
	Command  uint16
	BodySize uint16
}

// Serialize serializes Message into bytes.
func (em UsMessage) Serialize() ([]byte, error) {
	fmt.Println("Serialize")
	bodysize := em.Header.BodySize

	var testStruct = &em.Header
	Len := unsafe.Sizeof(UsMessageHeader{})
	testBytes := &reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(testStruct)),
		Cap:  int(Len),
		Len:  int(Len),
	}
	data := *(*[]byte)(unsafe.Pointer(testBytes))
	binary.BigEndian.PutUint16(data[0:2], em.Header.Type)
	binary.BigEndian.PutUint16(data[16:18], em.Header.Command)
	binary.BigEndian.PutUint16(data[18:20], em.Header.BodySize)
	if bodysize > 0 {
		bytes := append(data, em.Content...)
		return bytes, nil
	}
	return data, nil
}

// MessageNumber returns message type number.
func (em UsMessage) MessageNumber() uint16 {
	return 0x5553
}

// MessageTypeHandler 根据消息类型返回消息头剩余长度
func (em UsMessage) MessageTypeHandler(msgtype uint16) (uint, error) {
	fmt.Printf("MessageTypeHandler msgtype is %04x\n", msgtype)
	switch msgtype {
	case 0x5553: //'US'
		return uint(unsafe.Sizeof(UsMessageHeader{}) - 2), nil
	default:
		return 0, errors.New(fmt.Sprintf("undefined message type %04x", msgtype))
	}
}

//MessageHeadHandler 根据消息头返回消息体长度
func (em UsMessage) MessageHeadHandler(data []byte) (uint, error) {
	fmt.Println("MessageHeadHandler")
	if data == nil {
		return 0, tao.ErrNilData
	}

	Len := unsafe.Sizeof(UsMessageHeader{})
	if int(Len) > len(data) {
		return 0, tao.ErrNilData
	}

	typeBuf := bytes.NewReader(data[18:20])
	var msgLength uint16
	if err := binary.Read(typeBuf, binary.BigEndian, &msgLength); err != nil {
		return 0, err
	}

	return uint(msgLength), nil
}

// DeserializeMessage deserializes bytes into Message.
func (em UsMessage) DeserializeMessage(data []byte) (message tao.Message, err error) {
	fmt.Println("DeserializeMessage")
	if data == nil {
		return nil, tao.ErrNilData
	}
	Len := unsafe.Sizeof(UsMessageHeader{})
	if int(Len) > len(data) {
		return nil, tao.ErrWouldBlock
	}

	msgHeader := (*UsMessageHeader)(unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(&data)).Data))
	typeBuf := bytes.NewReader(data[0:2])
	binary.Read(typeBuf, binary.BigEndian, &msgHeader.Type)
	typeBuf = bytes.NewReader(data[16:18])
	binary.Read(typeBuf, binary.BigEndian, &msgHeader.Command)
	typeBuf = bytes.NewReader(data[18:20])
	binary.Read(typeBuf, binary.BigEndian, &msgHeader.BodySize)
	var msg UsMessage
	msg.Header = *msgHeader
	msg.Content = data[Len:]
	return msg, nil
}

// ProcessMessage process the logic of echo message.
func (em UsMessage) ProcessMessage(ctx context.Context, conn tao.WriteCloser) {
	fmt.Println("ProcessMessage")
	msg := tao.MessageFromContext(ctx).(UsMessage)
	holmes.Infof("receving message %s, Type is %04x, BodySize %d, Command %04x\n", string(msg.Content[:]), msg.Header.Type, msg.Header.BodySize, msg.Header.Command)

	switch conn.(type) {
	case *tao.ServerConn:
		c := conn.(*tao.ServerConn)
		c.Write(msg)
	case *tao.ClientConn:
		c := conn.(*tao.ClientConn)
		fmt.Println(c.NetID())
	default:
		fmt.Println("ProcessMessage type error")
	}
}

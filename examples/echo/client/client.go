package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/xu215740578/holmes"
	"github.com/xu215740578/tao"
	"github.com/xu215740578/tao/examples/echo"
)

func main() {
	tao.Register(echo.UsMessage{}.MessageNumber(), echo.UsMessage{}.DeserializeMessage, echo.UsMessage{}.ProcessMessage, echo.UsMessage{}.MessageTypeHandler, echo.UsMessage{}.MessageHeadHandler)

	c, err := net.Dial("tcp", "127.0.0.1:50099")
	if err != nil {
		holmes.Fatalln(err)
		fmt.Println(err)
	}

	onConnect := tao.OnConnectOption(func(conn tao.WriteCloser) bool {
		holmes.Infoln("on connect")
		_, ok := conn.(*tao.ClientConn)
		if !ok {
			fmt.Println("on connect !ok")
		}else{
			fmt.Println("on connect ok")
		}
		return true
	})

	onError := tao.OnErrorOption(func(conn tao.WriteCloser) {
		holmes.Infoln("on error")
		fmt.Println("on error")
	})

	onClose := tao.OnCloseOption(func(conn tao.WriteCloser) {
		holmes.Infoln("on close")
		fmt.Println("on close")
		os.Exit(1)
	})

	onMessage := tao.OnMessageOption(func(msg tao.Message, conn tao.WriteCloser) {
		echo := msg.(echo.UsMessage)
		fmt.Printf("%s\n", echo.Content)
	})

	conn := tao.NewClientConn(0, c, onConnect, onError, onClose, onMessage)

	header := echo.UsMessageHeader{
		Type: 0x5553,
		Version: 2,
		Encrypt: 0,
		Command: 12289,
		BodySize:uint16(len("hello, world")),
	}
	echo := echo.UsMessage{
		Header:    header,
		Content:   []byte("hello, world"),
	}

	conn.Start()

	for i := 0; i < 10; i++ {
		time.Sleep(60 * time.Millisecond)
		err := conn.Write(echo)
		if err != nil {
			holmes.Errorln(err)
		}
	}
	holmes.Debugln("hello")

	conn.RunEvery(5*time.Second, func(t time.Time, w tao.WriteCloser){
		conn.Write(echo)
	})
	time.Sleep(600*time.Second)
	conn.Close()
}


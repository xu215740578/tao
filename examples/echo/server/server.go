package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/xu215740578/holmes"
	"github.com/xu215740578/tao"
	"github.com/xu215740578/tao/examples/echo"
)

// EchoServer represents the echo server.
type EchoServer struct {
	*tao.Server
}

// NewEchoServer returns an EchoServer.
func NewEchoServer() *EchoServer {
	onConnect := tao.OnConnectOption(func(conn tao.WriteCloser) bool {
		holmes.Infoln("on connect")
		fmt.Println("on connect")
		s, ok := conn.(*tao.ServerConn)
		if !ok {
			fmt.Println("on connect !ok")
		} else {
			s.RunEvery(1*time.Second, func(t time.Time, w tao.WriteCloser) {
				s1 := w.(*tao.ServerConn)
				if time.Now().Unix()-s1.HeartBeat()/1000000000 > 15 {
					s1.Close()
				}
				fmt.Println(time.Now().Unix(), s1.HeartBeat())
			})
			fmt.Println("on connect ok", s.NetID())
		}
		return true
	})

	onClose := tao.OnCloseOption(func(conn tao.WriteCloser) {
		holmes.Infoln("closing client")
		s, _ := conn.(*tao.ServerConn)
		fmt.Println("closing client", s.NetID())
	})

	onError := tao.OnErrorOption(func(conn tao.WriteCloser) {
		holmes.Infoln("on error")
		s, _ := conn.(*tao.ServerConn)
		fmt.Println("on error", s.NetID())
	})

	onMessage := tao.OnMessageOption(func(msg tao.Message, conn tao.WriteCloser) {
		holmes.Infoln("receving message")
		s, _ := conn.(*tao.ServerConn)
		fmt.Println("receving message", s.NetID())
	})

	return &EchoServer{
		tao.NewServer(onConnect, onClose, onError, onMessage),
	}
}

func main() {
	defer holmes.Start().Stop()

	runtime.GOMAXPROCS(runtime.NumCPU())

	tao.Register(echo.UsMessage{}.MessageNumber(), echo.UsMessage{}.DeserializeMessage, echo.UsMessage{}.ProcessMessage, echo.UsMessage{}.MessageTypeHandler, echo.UsMessage{}.MessageHeadHandler)

	l, err := net.Listen("tcp", ":12345")
	if err != nil {
		holmes.Fatalf("listen error %v", err)
	}
	echoServer := NewEchoServer()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		echoServer.Stop()
	}()

	echoServer.Start(l)
}

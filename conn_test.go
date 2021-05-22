package pool

import (
	"HelloGo/basic/body"
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

var OPT = &Option{
	addr:        "0.0.0.0:3000",
	size:        3,
	readTimeout: 3 * time.Second,
	dialTimeout: 3 * time.Second,
	keepAlive:   1 * time.Second,
}

func createConn(opt *Option) *Conn {
	c, err := NewConn(opt)
	if err != nil {
		panic(err)
	}
	return c
}

func TestSendMsg(t *testing.T) {
	c := createConn(OPT)
	msg := &body.Message{Uid: "pixel-1", Val: "pixelpig!"}
	rec, err := c.Send(context.Background(), msg)
	if err != nil {
		t.Error(err)
	} else {
		t.Logf("rec1: %+v", <-rec)
	}

	//msg.Val = "another pig!"
	//rec2, err := c.SendInPool(context.Background(), msg)
	//if err != nil {
	//	t.Error(err)
	//} else {
	//	t.Logf("rec2: %+v", <-rec2)
	//}

	// 超时判断
	rec3, err := c.Send(context.Background(), msg)
	if err == nil {
		select {
		case resp := <-rec3:
			t.Logf("rec3: %+v", resp)
			return
		case <-time.After(time.Second * 1):
			t.Error("Wait for resp timeout!")
			return
		}
	} else {
		t.Error(err)
	}

	t.Log("finished")
}

func TestAliveCheck(t *testing.T) {
	conn, err := net.Dial("tcp", "127.0.0.1:3000")
	if err != nil {
		fmt.Println("dial failed:", err)
		os.Exit(1)
	}
	defer conn.Close()

	buffer := make([]byte, 512)

	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	if err := tcp.SetKeepAlive(true); err != nil {
		t.Error(err)
		return
	}
	// 30s之后开启状态检测
	if err = tcp.SetKeepAlivePeriod(30 * time.Second); err != nil {
		return
	}

	for {

		n, err := tcp.Read(buffer)
		if err != nil {
			fmt.Println("Read failed:", err)
			time.Sleep(1)
			continue
		}

		fmt.Println("count:", n, "msg:", string(buffer))
	}
}

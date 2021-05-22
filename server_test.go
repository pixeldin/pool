package pool

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"strconv"
	"testing"
	"time"
)

type Message struct {
	Uid string
	Val string
}

type Resp struct {
	Uid string
	Val string
	Ts  string
}

const TAG = "server: hello, "

func TestListenAndServer(t *testing.T) {
	ListenAndServer()
}

func ListenAndServer() {
	log.Print("Start server...")
	listen, err := net.Listen("tcp", "0.0.0.0:3000")
	if err != nil {
		log.Fatal("Listen failed. msg: ", err)
		return
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Printf("accept failed, err: %v", err)
			continue
		}
		go transfer(conn)
		//go handleConnection(conn)
	}
}

func transfer(conn net.Conn) {
	defer func() {
		remoteAddr := conn.RemoteAddr().String()
		log.Print("discard remove add:", remoteAddr)
		conn.Close()
	}()

	// 设置10秒关闭连接
	//conn.SetDeadline(time.Now().Add(10 * time.Second))

	for {
		var msg Message

		if err := json.NewDecoder(conn).Decode(&msg); err != nil && err != io.EOF {
			log.Printf("Decode from client err: %v", err)
			// todo... 仿照redis协议写入err前缀符号`-`，通知client错误处理
			return
		}

		if msg.Uid != "" || msg.Val != "" {
			//conn.Write([]byte(msg.Val))
			var rsp Resp
			rsp.Uid = msg.Uid
			rsp.Val = TAG + msg.Val

			rsp.Ts = strconv.FormatInt(time.Now().UnixNano(), 10)
			ser, _ := json.Marshal(rsp)

			// 模拟服务端耗时
			//time.Sleep(3 * time.Second)
			conn.Write(append(ser, '\n'))
		}
	}
}

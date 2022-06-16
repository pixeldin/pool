package pool

import (
	"HelloGo/basic/body"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
)

type IConn interface {
	Close() error
}

// Support for each connection
type Conn struct {
	addr    string
	tcp     *net.TCPConn // any conn base TCP, like redis/kafka/mysql
	ctx     context.Context
	writer  *bufio.Writer
	cnlFun  context.CancelFunc // to notify binding ctx done
	retChan *sync.Map          // a temp map for every connection's communication
	err     error
}

// check the implement of Close interface
var _ io.Closer = new(Conn)

func NewConn(opt *Option) (c *Conn, err error) {
	c = &Conn{
		addr:    opt.addr,
		retChan: new(sync.Map),
		//err: nil,
	}

	defer func() {
		if err != nil {
			if c != nil {
				c.Close()
			}
		}
	}()

	var conn net.Conn
	if conn, err = net.DialTimeout("tcp", opt.addr, opt.dialTimeout); err != nil {
		return
	} else {
		c.tcp = conn.(*net.TCPConn)
	}

	c.writer = bufio.NewWriter(c.tcp)

	//if err = c.tcp.SetKeepAlive(true); err != nil {
	if err = c.tcp.SetKeepAlive(false); err != nil {
		return
	}
	if err = c.tcp.SetKeepAlivePeriod(opt.keepAlive); err != nil {
		return
	}
	if err = c.tcp.SetLinger(0); err != nil {
		return
	}

	c.ctx, c.cnlFun = context.WithCancel(context.Background())

	// receive results asynchronously to the result set
	go receiveResp(c)

	return
}

// receive data from tcp Conn
func receiveResp(c *Conn) {
	scanner := bufio.NewScanner(c.tcp)
	for {
		select {
		case <-c.ctx.Done():
			// c.cnlFun() was call
			return
		default:
			if scanner.Scan() {
				rsp := new(body.Resp)
				if err := json.Unmarshal(scanner.Bytes(), rsp); err != nil {
					return
				}
				uid := rsp.Uid
				if load, ok := c.retChan.Load(uid); ok {
					c.retChan.Delete(uid)
					// message channel
					if ch, ok := load.(chan string); ok {
						ch <- "ts(ns): " + rsp.Ts + ", " + rsp.Val
						// closing at the writing side
						close(ch)
					}
				}
			} else {
				if scanner.Err() != nil {
					c.err = scanner.Err()
				} else {
					c.err = errors.New("scanner done")
				}
				c.Close()
				return
			}
		}
	}
}

// Close connection and make sure to close the relevant channel
func (c *Conn) Close() (err error) {
	if c.cnlFun != nil {
		c.cnlFun()
	}

	if c.tcp != nil {
		err = c.tcp.Close()
	}

	// close relevant chan
	if c.retChan != nil {
		c.retChan.Range(func(key, value interface{}) bool {
			// convert channel types according to specific services
			if ch, ok := value.(chan string); ok {
				close(ch)
			}
			return true
		})
	}
	return
}

/*
	Send the request and return the mapping channel,
	the return err for subsequent judgment whether
	to return the connection pool.
*/
func (c *Conn) Send(ctx context.Context, msg *body.Message) (ch chan string, err error) {
	ch = make(chan string)
	c.retChan.Store(msg.Uid, ch)
	js, _ := json.Marshal(msg)

	_, err = c.writer.Write(js)
	if err != nil {
		return
	}

	err = c.writer.Flush()
	//c.tcp.CloseWrite()
	return
}

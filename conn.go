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

// Conn 对应每个连接
type Conn struct {
	addr    string       // 地址
	tcp     *net.TCPConn // tcp连接实例, 可以是其他类型
	ctx     context.Context
	writer  *bufio.Writer
	cnlFun  context.CancelFunc // 用于通知ctx结束
	retChan *sync.Map          // 存放通道结果集合的map, 属于统一连接
	err     error
}

// 检验是否实现Close接口
var _ io.Closer = new(Conn)

func NewConn(opt *Option) (c *Conn, err error) {
	// 初始化连接
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

	// 拨号
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

	// 异步接收结果到相应的结果集
	go receiveResp(c)

	return
}

// receiveResp 接收tcp连接的数据
func receiveResp(c *Conn) {
	scanner := bufio.NewScanner(c.tcp)
	for {
		select {
		case <-c.ctx.Done():
			// c.cnlFun() 被执行了, 如连接池关闭
			return
		default:
			if scanner.Scan() {
				// 读取数据
				rsp := new(body.Resp)
				if err := json.Unmarshal(scanner.Bytes(), rsp); err != nil {
					return
				}
				uid := rsp.Uid
				if load, ok := c.retChan.Load(uid); ok {
					c.retChan.Delete(uid)
					// 消息通道
					if ch, ok := load.(chan string); ok {
						ch <- "ts(ns): " + rsp.Ts + ", " + rsp.Val
						// 在写入端关闭
						close(ch)
					}
				}
			} else {
				// 错误, 合并了EOF
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

// Close 关闭连接, 关闭消息通道
func (c *Conn) Close() (err error) {
	// 执行善后
	if c.cnlFun != nil {
		c.cnlFun()
	}

	// 关闭tcp连接
	if c.tcp != nil {
		err = c.tcp.Close()
	}

	// 关闭消息通道
	if c.retChan != nil {
		c.retChan.Range(func(key, value interface{}) bool {
			// 根据具体业务转换通道类型
			if ch, ok := value.(chan string); ok {
				close(ch)
			}
			return true
		})
	}
	return
}

/*
	SendInPool 发送请求, 返回具体业务通道
	注意如果入参的msg消息体是interface{}类型, 最好根据业务进行
	类型断言校验, 避免server端解析出错，返回err值用于后续判断
	是否归还连接池。
*/
func (c *Conn) Send(ctx context.Context, msg *body.Message) (ch chan string, err error) {
	ch = make(chan string)
	c.retChan.Store(msg.Uid, ch)
	// 请求
	js, _ := json.Marshal(msg)

	_, err = c.writer.Write(js)
	if err != nil {
		return
	}

	err = c.writer.Flush()
	// 连接不关闭, 后续可以放入连接池
	//c.tcp.CloseWrite()
	return
}

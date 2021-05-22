# pool
一个tcp连接池的简单实现。

```go
/*
	pool 具备行为
*/
type IPool interface {
	Close() error
	Get() (c *Conn, err error)
	Put(c *Conn, err error)
}
```

## Demo示例
```go
var opt = &Option{
	addr: "127.0.0.1:3000",
	// 初始化5个连接
	size:        5,
	readTimeout: 30 * time.Second,
	dialTimeout: 5 * time.Second,
	keepAlive:   30 * time.Second,
}

func TestNewPool(t *testing.T) {
	pool, err := NewPool(opt)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		go func(id int) {
			if err := SendInPool(pool, "Uid-"+strconv.Itoa(id)); err != nil {
				log.Print("Send in pool err: ", err)
			}
			// 注意闭包的id传入值
		}(i)
	}

	for {
		time.Sleep(1)
	}
}

func SendInPool(p *Pool, uid string) (err error) {
	var c *Conn
	if c, err = p.Get(); err != nil {
		return
	}
	defer p.Put(c, err)
	msg := &body.Message{Uid: uid, Val: "pixelpig!"}
	rec, err := c.Send(context.Background(), msg)
	if err != nil {
		log.Print(err)
	} else {
		log.Print(uid, ", Msg: ", <-rec)
	}
	return
}
```

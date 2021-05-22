package pool

import (
	"container/list"
	"log"
	"sync"
	"time"
)

type Option struct {
	addr        string
	size        int
	readTimeout time.Duration
	dialTimeout time.Duration
	keepAlive   time.Duration
}

type Pool struct {
	*Option
	idle    *list.List  // 空闲队列(双向)链表
	actives int         // 总连接数
	mtx     *sync.Mutex // 同步锁
	cond    *sync.Cond  // 用于阻塞/唤醒
}

// NewPool 创建连接池,初始化连接队列,更新队列大小
func NewPool(opt *Option) (p *Pool, err error) {
	idle := list.New()
	var conn *Conn
	for i := 0; i < opt.size; i++ {
		conn, err = NewConn(opt)
		if err == nil {
			// 加入队列
			idle.PushBack(conn)
		}
		// whether close all idle conn when one of err occurs?
	}

	mutx := new(sync.Mutex)
	cond := sync.NewCond(mutx)

	p = &Pool{
		opt,
		idle,
		idle.Len(),
		mutx,
		cond,
	}
	return
}

/*
	Get 获取连接:
	空闲列表是否有库存:
		- 没有
			- 连接数是否达到上限
				- 是, 解锁, 阻塞等待唤醒
				- 否, 创建连接
		- 从队头出队获取
*/
func (p *Pool) Get() (c *Conn, err error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	// 如果当前活跃大于限制数量, 阻塞等待
	for p.idle.Len() == 0 && p.actives >= p.size {
		log.Print("idle size full, blocking...")
		// 让出使用权并且释放mutex锁
		p.cond.Wait()
	}
	// 空闲列表如果有且连接, 则从空闲队头获取
	if p.idle.Len() > 0 {
		c = p.idle.Remove(p.idle.Front()).(*Conn)
	} else {
		// 创建连接
		c, err = NewConn(p.Option)
		if err == nil {
			p.actives++
		}
	}
	return
}

/*
  Put() 归还连接
  - 连接是否异常
	- 是, 关闭异常连接
	- 否, 归还连接至队尾
  - 更新占用连接数, 唤醒等待方
*/
func (p *Pool) Put(c *Conn, err error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if err != nil {
		if c != nil {
			c.Close()
		}
	} else {
		p.idle.PushBack(c)
	}
	p.actives--
	p.cond.Signal()
}

// Close 遍历链表关闭连接
func (p *Pool) Close() (err error) {
	//p.mtx.Lock()
	//defer p.mtx.Unlock()
	for n := p.idle.Front(); n == nil; n = n.Next() {
		n.Value.(*Conn).Close()
	}
	return
}

// Reset 重置连接池
func (p *Pool) Reset() {
	newIdle := list.New()
	var conn *Conn
	var err error
	for i := 0; i < p.Option.size; i++ {
		conn, err = NewConn(p.Option)
		if err != nil {
			for e := newIdle.Front(); e != nil; e = e.Next() {
				e.Value.(*Conn).Close()
			}
			return
		}
		newIdle.PushBack(conn)
	}

	var oldIdle *list.List
	p.mtx.Lock()
	oldIdle = p.idle
	p.idle = newIdle
	p.actives = newIdle.Len()
	p.mtx.Unlock()

	if oldIdle != nil {
		for e := oldIdle.Front(); e != nil; e = e.Next() {
			e.Value.(*Conn).Close()
		}
	}
}

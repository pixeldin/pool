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
	idle    *list.List // idle doubly linked list
	actives int        // total connection count
	mtx     *sync.Mutex
	cond    *sync.Cond
}

// NewPool initialization of the pool queue
func NewPool(opt *Option) (p *Pool, err error) {
	idle := list.New()
	var conn *Conn
	for i := 0; i < opt.size; i++ {
		conn, err = NewConn(opt)
		if err == nil {
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
	Get() :
	Is the free list in idle queue:
		- no
			- Whether the number of connections has reached the upper limit
				- yes, unlocked, blocked waiting to wake up
				- no, Create a connection
		- pop from head of the queue
*/
func (p *Pool) Get() (c *Conn, err error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	// If the current activity over the limit number, block and wait
	for p.idle.Len() == 0 && p.actives >= p.size {
		log.Print("idle size full, blocking...")
		// cancel possessing and release the mutex lock,
		// it will wake up automatically when the for condition does not hold
		p.cond.Wait()
	}

	if p.idle.Len() > 0 {
		c = p.idle.Remove(p.idle.Front()).(*Conn)
	} else {
		c, err = NewConn(p.Option)
		if err == nil {
			p.actives++
		}
	}
	return
}

/*
  Put()
  - Is the connection alive?
	- no, close it
	- yes, return link to end of line
  - Update the number of occupied connections, wake up the waiting side
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

func (p *Pool) Close() (err error) {
	for n := p.idle.Front(); n == nil; n = n.Next() {
		n.Value.(*Conn).Close()
	}
	return
}

// Reset the idle list
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

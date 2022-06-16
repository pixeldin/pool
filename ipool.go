package pool

type IPool interface {
	Close() error
	Get() (c *Conn, err error)
	Put(c *Conn, err error)
}

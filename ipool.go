package pool

/*
	pool 具备行为
*/
type IPool interface {
	Close() error
	Get() (c *Conn, err error)
	Put(c *Conn, err error)
}

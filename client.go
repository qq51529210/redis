package redis

import (
	"errors"
	"net"
	"sync"
	"time"
)

var (
	errClosedClient = errors.New("client has been closed")
)

// Redis command client with connection pool.
type Client struct {
	cond *sync.Cond
	// Whether this client is valid.
	ok bool
	// Use for create a new net.Conn.
	newConn func(string) (net.Conn, error)
	// Server address.
	host string
	// Index of db,every command will use this db.
	dbIndex int
	// IO read timeout,millisecond.
	readTimeout time.Duration
	// IO write timeout,millisecond.
	writeTimeout time.Duration
	// Connection pool.
	connPool []*conn
}

// Init data for NewClient().
type ClientConfig struct {
	// Redis server listen address,it will pass to newConn().Default is "localhost:6379"
	Host string `json:"host"`
	// Index of db which every command whill choose.
	DB int `json:"db"`
	// Maximum connections.
	MaxConn int `json:"maxConn"`
	// IO read timeout,millisecond.
	ReadTimeout int `json:"readTimeout"`
	// IO write timeout,millisecond.
	WriteTimeout int `json:"writeTimeout"`
}

// Create a redis command client. If arg newConn is nil,use net.Dial() instead.
func NewClient(dialFunc func(string) (net.Conn, error), cfg *ClientConfig) *Client {
	c := new(Client)
	c.cond = sync.NewCond(new(sync.Mutex))
	c.ok = true
	// Host
	c.host = cfg.Host
	if c.host == "" {
		c.host = "127.0.0.1:6379"
	}
	// DB
	if cfg.DB > 0 {
		c.dbIndex = cfg.DB
	}
	// ReadTimeout
	if cfg.ReadTimeout > 0 {
		c.readTimeout = time.Duration(cfg.ReadTimeout)
	}
	c.readTimeout *= time.Millisecond
	// WriteTimeout
	if cfg.WriteTimeout > 0 {
		c.writeTimeout = time.Duration(cfg.WriteTimeout)
	}
	c.writeTimeout *= time.Millisecond
	// MaxConn
	if cfg.MaxConn < 1 {
		c.connPool = make([]*conn, 1)
	} else {
		c.connPool = make([]*conn, cfg.MaxConn)
	}
	// Function newConn
	if dialFunc == nil {
		if c.readTimeout > 0 {
			c.newConn = func(host string) (net.Conn, error) {
				return net.DialTimeout("tcp", host, c.readTimeout)
			}
		} else {
			c.newConn = func(host string) (net.Conn, error) {
				return net.Dial("tcp", host)
			}
		}
	} else {
		c.newConn = dialFunc
	}
	// Init conn pool.
	for i := 0; i < len(c.connPool); i++ {
		c.connPool[i] = new(conn)
		c.connPool[i].free = true
	}
	return c
}

// Close this client,and all net.Conn.
func (c *Client) Close() error {
	// Change to closed state.
	c.cond.L.Lock()
	if !c.ok {
		c.cond.L.Unlock()
		return errClosedClient
	}
	c.ok = false
	c.cond.L.Unlock()
	// Close all net.Conn.
	for i := 0; i < len(c.connPool); i++ {
		if c.connPool[i].Conn != nil {
			c.connPool[i].Close()
		}
	}
	return nil
}

// Write command to server,and read response from server.
// Example: Client.Cmd("set", "a", 1).
// Args data type:
// 	int,float,string,[]byte -> string.
//	struct,[]interface{} -> json.
// Return value data type could be one of [nil, string, int64, []interface{}].
// Return error could be network error or server error message.
func (c *Client) Cmd(args ...interface{}) (interface{}, error) {
	// Get free conn.
	conn, err := c.getConn()
	if err != nil {
		c.onConnError(conn, err)
		return nil, err
	}
	var value interface{}
	value, err = c.writeCmd(conn, args...)
	if err != nil {
		c.onConnError(conn, err)
		return nil, err
	}
	// Free conn.
	c.putConn(conn)
	return value, nil
}

// Get free conn from pool.
func (c *Client) getConn() (*conn, error) {
	// Lock and block this routinue.
	c.cond.L.Lock()
	for c.ok {
		// Check free conn.
		for _, conn := range c.connPool {
			if conn.free {
				conn.free = false
				c.cond.L.Unlock()
				return conn, c.checkConn(conn)
			}
		}
		// There is no free conn,wait for free one.
		c.cond.Wait()
	}
	c.cond.L.Unlock()
	// Client has been closed.
	return nil, errClosedClient
}

// Put conn into pool.
func (c *Client) putConn(conn *conn) {
	c.cond.L.Lock()
	if c.ok {
		conn.free = true
		c.cond.L.Unlock()
		// Notify other routine,there's a free conn.
		c.cond.Signal()
		return
	}
	c.cond.L.Unlock()
}

// If net.Conn is nil,create a new one and select db.
func (c *Client) checkConn(conn *conn) (err error) {
	// If net.Conn is invalid,create a new one.
	if conn.Conn != nil {
		return nil
	}
	conn.Conn, err = c.newConn(c.host)
	if err != nil {
		return err
	}
	// Select db,becase redis default db is 0,so write command when db>0.
	if c.dbIndex > 0 {
		_, err = c.writeCmd(conn, "select", c.dbIndex)
	}
	return err
}

// If there's any error,free conn.
func (c *Client) onConnError(conn *conn, err error) {
	if conn.Conn != nil {
		conn.Conn.Close()
		conn.Conn = nil
	}
	c.putConn(conn)
}

func (c *Client) writeCmd(conn *conn, args ...interface{}) (interface{}, error) {
	conn.buff = conn.buff[:0]
	// Step 1: write command count into buffer.
	conn.WriteCount(int64(len(args)))
	for _, a := range args {
		// Step 2: write command into buffer.
		conn.WriteValue(a)
	}
	var err error
	// If c.writeTimeout was set.
	if c.writeTimeout > 0 {
		err = conn.Conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
		if err != nil {
			return nil, err
		}
	}
	// Write buffer to server.
	_, err = conn.Conn.Write(conn.buff)
	if err != nil {
		return nil, err
	}
	// If c.readTimeout was set.
	if c.readTimeout > 0 {
		err := conn.Conn.SetReadDeadline(time.Now().Add(c.readTimeout))
		if err != nil {
			return nil, err
		}
	}
	// Read and parse response.
	conn.newLineIdx, conn.parsedIdx, conn.resLenIdx = 0, 0, 0
	conn.buff = conn.buff[:cap(conn.buff)]
	var value interface{}
	value, err = conn.ReadValue()
	if err != nil {
		return nil, err
	}
	return value, err
}

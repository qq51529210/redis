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
func NewClient(newConn func(string) (net.Conn, error), cfg *ClientConfig) *Client {
	// Create and initialize.
	c := new(Client)
	c.cond = sync.NewCond(new(sync.Mutex))
	c.ok = true
	c.newConn = newConn
	if c.newConn == nil {
		c.newConn = func(host string) (net.Conn, error) {
			return net.Dial("tcp", host)
		}
	}
	c.host = cfg.Host
	if c.host == "" {
		c.host = "localhost:6379"
	}
	c.dbIndex = maxInt(cfg.DB, 0)
	c.readTimeout = time.Duration(maxInt(cfg.ReadTimeout, 0)) * time.Millisecond
	c.writeTimeout = time.Duration(maxInt(cfg.WriteTimeout, 0)) * time.Millisecond
	c.connPool = make([]*conn, maxInt(cfg.MaxConn, 1))
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
// Example: Client.Cmd("set", "test", "ok").
// If cmd is struct,it will convert to json string.
// Return value data type could be one of [nil, string, int64, []interface{}].
// Return error could be network error or server error message.
func (c *Client) Cmd(cmd ...interface{}) (interface{}, error) {
	// Get free conn.
	conn, err := c.getConn()
	if err != nil {
		c.onConnError(conn, err)
		return nil, err
	}
	// Step 1: write command count into buffer.
	conn.WriteCmdCount(int64(len(cmd)))
	for _, a := range cmd {
		// Step 2: write command into buffer.
		conn.WriteValue(a)
	}
	// Write buffer to server.
	err = c.writeRequest(conn)
	if err != nil {
		c.onConnError(conn, err)
		return nil, err
	}
	// Read and parse response.
	var val interface{}
	val, err = c.readResponse(conn)
	if err != nil {
		c.onConnError(conn, err)
		return nil, err
	}
	// Free conn.
	c.putConn(conn)
	return val, nil
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
	// Chose db,becase redis default db is 0,so write command when db>0.
	if c.dbIndex > 0 {
		// "select x"
		conn.WriteCmdCount(2)
		conn.WriteString("select")
		conn.WriteInt(int64(c.dbIndex))
		_, err = conn.Conn.Write(conn.buff)
		if err != nil {
			return err
		}
		// "+OK" or "-Error message"
		_, err = conn.ReadValue()
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

// Write conn buffer to server.
func (c *Client) writeRequest(conn *conn) (err error) {
	// If c.writeTimeout was set.
	if c.writeTimeout > 0 {
		err = conn.Conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
		if err != nil {
			return
		}
	}
	_, err = conn.Conn.Write(conn.buff)
	return
}

// Read and parse result.
func (c *Client) readResponse(conn *conn) (interface{}, error) {
	// If c.readTimeout was set.
	if c.readTimeout > 0 {
		err := conn.Conn.SetReadDeadline(time.Now().Add(c.readTimeout))
		if err != nil {
			return nil, err
		}
	}
	return conn.ReadValue()
}

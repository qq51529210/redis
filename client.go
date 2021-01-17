package redis

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

var (
	errClosed         = errors.New("client has been closed")
	errReadString     = errors.New("read string from server error")
	DefaultBufferSize = 128
)

// 返回最大的
func maxInt(i1, i2 int) int {
	if i1 > i2 {
		return i1
	}
	return i2
}

// 解析整数
func parseInt(b []byte) (int64, error) {
	var n int64
	if len(b) > 0 {
		i := 0
		if b[i] == '-' {
			i++
		}
		if '0' <= b[i] && b[i] <= '9' {
			n = int64(b[i] - '0')
			i++
		} else {
			return n, fmt.Errorf("parse int invalid <%q>", b[i])
		}
		for ; i < len(b); i++ {
			if '0' <= b[i] && b[i] <= '9' {
				n *= 10
				n += int64(b[i] - '0')
			} else {
				return n, fmt.Errorf("parse int invalid <%q>", b[i])
			}
		}
		if b[0] == '-' {
			n = 0 - n
		}
	}
	return n, nil
}

func NewClient(dial func() (net.Conn, error), db, max int, rto, wto time.Duration) *Client {
	p := new(Client)
	p.cond = sync.NewCond(new(sync.Mutex))
	p.ok = true
	p.dial = dial
	if p.dial == nil {
		p.dial = func() (net.Conn, error) {
			return net.Dial("tcp", "127.0.0.1:6379")
		}
	}
	p.db = maxInt(db, 0)
	p.rto = rto
	p.wto = wto
	max = maxInt(max, 1)
	p.free = make([]*conn, 0)
	p.all = make([]*conn, 0, max)
	return p
}

type conn struct {
	net.Conn // 底层连接
	buffer   // 缓存
}

// 带连接池的客户端
type Client struct {
	cond *sync.Cond
	db   int                      // 对应的库的索引
	ok   bool                     // 是否有效
	dial func() (net.Conn, error) // 获取底层连接函数
	rto  time.Duration            // io读超时
	wto  time.Duration            // io写超时
	free []*conn                  // 当前空闲可用的连接
	all  []*conn                  // 保存所有的连接
}

// 关闭
func (c *Client) Close() error {
	// 改变状态
	c.cond.L.Lock()
	if !c.ok {
		c.cond.L.Unlock()
		return errClosed
	}
	c.ok = false
	c.cond.L.Unlock()
	// 关闭所有连接
	for i := 0; i < len(c.all); i++ {
		if c.all[i].Conn != nil {
			_ = c.all[i].Close()
		}
	}
	return nil
}

// 发送命令，并获取结果
func (c *Client) Do(args ...interface{}) (interface{}, error) {
	// 获取conn
	conn, err := c.getConn()
	if err != nil {
		c.onError(conn, err)
		return nil, err
	}
	// 写入命令
	conn.CmdCount(int64(len(args)))
	for _, a := range args {
		conn.Value(a)
	}
	// 请求
	err = c.write(conn)
	if err != nil {
		c.onError(conn, err)
		return nil, err
	}
	// 相应
	var val interface{}
	val, err = c.read(conn)
	if err != nil {
		c.onError(conn, err)
		return nil, err
	}
	// 回收
	c.putConn(conn)
	// 返回
	return val, nil
}

func (c *Client) check(conn *conn) error {
	if conn.Conn != nil {
		return nil
	}
	// 建立连接
	var err error
	conn.Conn, err = c.dial()
	if err != nil {
		return err
	}
	// 选择库
	if c.db > 0 {
		conn.CmdCount(2)
		conn.String("select")
		conn.Int(int64(c.db))
		// 发送
		_, err = conn.Write(conn.buf)
		if err != nil {
			return err
		}
		// 读取"OK"
		_, err = c.read(conn)
		if err != nil {
			return err
		}
		conn.buf = conn.buf[:0]
	}
	return nil
}

func (c *Client) getConn() (*conn, error) {
	c.cond.L.Lock()
	for c.ok {
		// 是否有可用的连接
		if len(c.free) > 0 {
			n := len(c.free) - 1
			conn := c.free[n]
			c.free = c.free[:n]
			c.cond.L.Unlock()
			return conn, c.check(conn)
		}
		// 是否达到了最大的连接
		if len(c.all) < cap(c.all) {
			conn := new(conn)
			conn.buf = make([]byte, DefaultBufferSize)
			c.all = append(c.all, conn)
			c.cond.L.Unlock()
			return conn, c.check(conn)
		}
		// 等待空闲的
		c.cond.Wait()
	}
	c.cond.L.Unlock()
	return nil, errClosed
}

func (c *Client) putConn(conn *conn) {
	c.cond.L.Lock()
	if c.ok {
		c.free = append(c.free, conn)
		c.cond.L.Unlock()
		// 通知getConn()有新的可用conn
		c.cond.Signal()
		return
	}
	c.cond.L.Unlock()
}

func (c *Client) onError(conn *conn, err error) {
	if netErr, ok := err.(net.Error); ok &&
		!netErr.Timeout() && !netErr.Temporary() {
		if conn.Conn != nil {
			_ = conn.Conn.Close()
			conn.Conn = nil
		}
	}
	// 回收
	c.putConn(conn)
}

func (c *Client) write(conn *conn) (err error) {
	// 设置超时
	if c.wto > 0 {
		err = conn.SetWriteDeadline(time.Now().Add(c.wto))
		if err != nil {
			return
		}
	}
	// 发送
	_, err = conn.Write(conn.buf)
	return
}

func (c *Client) read(conn *conn) (interface{}, error) {
	// 设置超时
	if c.rto > 0 {
		err := conn.SetReadDeadline(time.Now().Add(c.rto))
		if err != nil {
			return nil, err
		}
	}
	// 读取，解析，返回响应数据
	conn.i1, conn.i2, conn.len = 0, 0, 0
	return c.readValue(conn)
}

func (c *Client) readValue(conn *conn) (interface{}, error) {
	line, err := c.readLine(conn)
	if err != nil {
		return nil, err
	}
	// 数据类型
	switch line[0] {
	case '+': // 简单字符串
		return string(line[1 : len(line)-2]), nil
	case '-': // 错误
		return nil, errors.New(string(line[1:]))
	case ':': // 整数
		return parseInt(line[1:])
	case '$': // 字符串
		var length int64 // 长度
		length, err = parseInt(line[1 : len(line)-2])
		if err != nil {
			return nil, err
		}
		// nil对象
		if length < 0 {
			return nil, nil
		}
		// 再读n+2("\r\n")个字节
		length += 2
		if conn.len-conn.i2 < int(length) {
			var n int
			for length > 0 {
				n, err = conn.Conn.Read(conn.buf[conn.len:])
				if err != nil {
					return nil, err
				}
				conn.len += n
				length -= int64(n)
				if conn.len == len(conn.buf) {
					conn.resize()
				}
			}
		}
		var ok bool
		line, ok = conn.ReadLine()
		if !ok {
			return nil, errReadString
		}
		return string(line[1 : len(line)-2]), nil
	case '*': // 数组
		var count int64 // 个数
		count, err = parseInt(line[1 : len(line)-2])
		if err != nil {
			return nil, err
		}
		var array []interface{}
		// 读取元素
		for i := int64(0); i < count; i++ {
			var a interface{}
			a, err = c.readValue(conn)
			if err != nil {
				return nil, err
			}
			array = append(array, a)
		}
		return array, nil
	default:
		return nil, fmt.Errorf("invalid data type <%q> from server", conn.buf[0])
	}
}

func (c *Client) readLine(conn *conn) ([]byte, error) {
	b, o := conn.ReadLine()
	if o {
		return b, nil
	}
	// 读数据
	var err error
	var n int
	for {
		n, err = conn.Conn.Read(conn.buf[conn.len:])
		if err != nil {
			return nil, err
		}
		conn.len += n
		b, o = conn.ReadLine()
		if o {
			return b, nil
		}
	}
}

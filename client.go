package redis

import (
	"fmt"
	"net"
	"sync"
	"time"
)

func NewClient(dial DialFunc, maxConn int, rto, wto time.Duration) *Client {
	p := new(Client)
	p.cond = sync.NewCond(&sync.Mutex{})
	p.valid = true
	p.dial = dial
	p.rto = rto
	p.wto = wto
	maxConn = maxInt(maxConn, 1)
	p.free = make([]net.Conn, 0, maxConn)
	p.all = make([]net.Conn, 0, maxConn)
	return p
}

// Redis接口实现，表示一个客户端对象，带连接池
type Client struct {
	cond  *sync.Cond    // 同步锁
	valid bool          // 是否关闭
	free  []net.Conn    // 当前空闲可用的连接，栈
	all   []net.Conn    // 保存所有的连接
	dial  DialFunc      // 获取底层连接函数
	rto   time.Duration // io读超时
	wto   time.Duration // io写超时
}

// 开始一条新的命令
func (c *Client) NewCmd(cmd ...string) *Cmd {
	_cmd := getCmd()
	for i := 0; i < len(cmd); i++ {
		_cmd.String(cmd[i])
	}
	return _cmd
}

// 执行命令
// switch value.(type)
// case int64:
// case error:
// case string:
// case []interface:
func (c *Client) DoCmd(cmd *Cmd) (value interface{}, err error) {
	var conn net.Conn
	// net.Conn
	conn, err = c.getConn()
	if err != nil {
		putCmd(cmd)
		return
	}
	// 请求
	{
		// 设置超时
		err = conn.SetWriteDeadline(time.Now().Add(c.wto))
		if err != nil {
			c.onNetError(conn, cmd, err)
			return
		}
		// 发送
		_, err = cmd.WriteTo(conn)
		if err != nil {
			c.onNetError(conn, cmd, err)
			return
		}
	}
	// 响应
	{
		// 设置超时
		err = conn.SetReadDeadline(time.Now().Add(c.rto))
		if err != nil {
			c.onNetError(conn, cmd, err)
			return
		}
		// 读取，解析，返回响应数据
		cmd.res = cmd.res[:0]
		cmd.idx = 0
		value, err = cmd.ReadValue(conn)
		if err != nil {
			c.onNetError(conn, cmd, err)
			return
		}
	}
	putCmd(cmd)
	c.putConn(conn)
	return
}

// 执行命令
func (c *Client) Do(cmd ...string) (value interface{}, err error) {
	_cmd := c.NewCmd(cmd...)
	return c.DoCmd(_cmd)
}

// 关闭
func (c *Client) Close() error {
	// 改变状态
	c.cond.L.Lock()
	if !c.valid {
		c.cond.L.Unlock()
		return useClosedRedis
	}
	c.valid = false
	c.cond.L.Unlock()
	// 关闭所有连接
	for i := 0; i < len(c.all); i++ {
		c.all[i].Close()
	}
	return nil
}

// DoCmd()，处理网络错误的代码
func (c *Client) onNetError(conn net.Conn, cmd *Cmd, err error) {
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() || netErr.Temporary() {
			c.putConn(conn)
		} else {
			conn.Close()
		}
		putCmd(cmd)
	}
}

// 获取一个可用的连接，返回conn或者
func (c *Client) getConn() (net.Conn, error) {
	c.cond.L.Lock()
	for c.valid {
		// 是否有可用的连接
		n := len(c.free) - 1
		if n > 0 {
			// 最后一个
			conn := c.free[n]
			c.free = c.free[:n]
			c.cond.L.Unlock()
			return conn, nil
		}
		// 是否可以添加新net.Conn
		if len(c.all) < cap(c.all) {
			conn, err := c.dial()
			if err == nil {
				c.all = append(c.all, conn)
			}
			c.cond.L.Unlock()
			return conn, err
		}
		// 等待空闲的
		c.cond.Wait()
	}
	c.cond.L.Unlock()
	return nil, useClosedRedis
}

// 回收连接
func (c *Client) putConn(conn net.Conn) {
	c.cond.L.Lock()
	if c.valid {
		c.free = append(c.free, conn)
		c.cond.L.Unlock()
		// 通知getConn()有新的可用conn
		c.cond.Signal()
		return
	}
	c.cond.L.Unlock()
}

// 返回包含\r\n的数据
func indexEndLine(buf []byte) int {
	for i := 1; i < len(buf); i++ {
		if buf[i-1] == '\r' && buf[i] == '\n' {
			return i + 1
		}
	}
	return -1
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
			return n, fmt.Errorf("parse int invalid char %d", b[i])
		}
		for ; i < len(b); i++ {
			if '0' <= b[i] && b[i] <= '9' {
				n *= 10
				n += int64(b[i] - '0')
			} else {
				return n, fmt.Errorf("parse int invalid char %d", b[i])
			}
		}
		if b[0] == '-' {
			n = 0 - n
		}
	}
	return n, nil
}

package redis

import (
	"errors"
	"net"
	"time"
)

// 表示一个连接对象
type conn struct {
	conn  net.Conn // 底层的连接
	valid bool     // 当前连接是否有效
	rbuf  []byte   // 读缓存
}

// 看Redis.Cmd函数说明
// 参数dial在Redis.Cmd函数中指定
func (this *conn) Cmd(ctx *dialContext, request *Request, response *Response) (rn int, wn int, e error) {
	// 检查连接
	if !this.valid {
		// 失败重试
		for i := 0; i < ctx.retry; i++ {
			this.conn, e = ctx.dial()
			if nil == e {
				this.valid = true
				break
			}
		}
	}
	// 发送请求
	wn, e = this.write(request, ctx.timeout)
	if nil != e {
		return
	}
	// 接收响应
	response.reset()
	rn, e = this.read(response, ctx.timeout)
	return
}

// 发送请求
func (this *conn) write(request *Request, timeout time.Duration) (wn int, e error) {
	if timeout > 0 {
		e = this.conn.SetWriteDeadline(time.Now().Add(timeout))
		if nil != e {
			return
		}
	}

	n := 0
	if request.n > 1 {
		request.c[0] = '*'
		n = formatInt(request.c[1:], int64(request.n)) + 1
		n += copy(request.c[n:], crlf)
		n, e = this.conn.Write(request.c[:n])
		wn += n
		if nil != e {
			return
		}
	}

	if timeout > 0 {
		e = this.conn.SetWriteDeadline(time.Now().Add(timeout))
		if nil != e {
			return
		}
	}
	n, e = this.conn.Write(request.b)
	wn += n
	return
}

// 接收响应
func (this *conn) read(response *Response, timeout time.Duration) (rn int, e error) {
	// response.b中新行的起始索引
	idx := response.i[len(response.i)-1]
	n := 0
	// 需要再读数据
	if idx == len(response.b) {
		n, e = this.readData(response, timeout)
		rn += n
		if nil != e {
			return
		}
	}
	// 消息类型
	switch response.b[idx] {
	case '+', '-', ':':
		// 这些消息都是单行
		for {
			// 是否已经读完一行
			if response.crlf(idx+1) > 0 {
				return
			}
			// 继续读
			n, e = this.readData(response, timeout)
			rn += n
			if nil != e {
				return
			}
		}
	case '$':
		// 先读字符串长度
		i1 := idx + 1
		i2 := i1
		i2, n, e = this.readLine(response, timeout, i2)
		rn += n
		if nil != e {
			return
		}
		m := parseInt(response.b[i1:i2-1])
		if m > 0 {
			// 有值，再读指定的长度+crlf
			m = m + idx + 2
			for m > 0 {
				i2, n, e = this.readLine(response, timeout, i2+1)
				rn += n
				m -= i2
				if nil != e {
					return
				}
			}
		}
	case '*':
		// 先读完数组长度行
		i1 := idx + 1
		i2 := i1
		i2, n, e = this.readLine(response, timeout, i2)
		rn += n
		if nil != e {
			return
		}
		count := parseInt(response.b[i1:i2-1])
		// 读数组的元素
		for j := 0; j < count; j++ {
			n, e = this.read(response, timeout)
			rn += n
			if nil != e {
				return
			}
		}
	default:
		e = errors.New("invalid cmd type from redis server")
	}
	return
}

// 从net.Conn中读取数据
func (this *conn) readData(response *Response, t time.Duration) (n int, e error) {
	if t > 0 {
		e = this.conn.SetReadDeadline(time.Now().Add(t))
		if nil != e {
			return
		}
	}
	n, e = this.conn.Read(this.rbuf)
	if nil != e {
		return
	}
	response.b = append(response.b, this.rbuf[:n]...)
	return
}

// 扫描第一行数据的crlf索引，数据不足则从net.Conn中读取
func (this *conn) readLine(response *Response, timeout time.Duration, i int) (int, int, error) {
	var n, rn int
	var e error
	for {
		i = response.crlf(i)
		if i > 0 {
			break
		}
		i = len(response.b)
		// 继续读
		n, e = this.readData(response, timeout)
		rn += n
		if nil != e {
			return i, rn, e
		}
	}
	return i, rn, nil
}

// 关闭这个连接
// 在Redis.Close函数中调用
func (this *conn) Close() error {
	if this.valid {
		this.valid = false
		return this.conn.Close()
	}
	return nil
}

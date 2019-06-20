package redis

import (
	"errors"
	"net"
	"time"
)

// 表示一个连接对象
type conn struct {
	conn   net.Conn // 底层的连接
	valid  bool     // 当前连接是否有效
	reConn int      // 失败重新连接次数
	rbuf   []byte   // 读缓存
}

// 看Redis.Cmd函数说明
// 参数dial在Redis.Cmd函数中指定
func (this *conn) Cmd(dial DialFunc, timeout time.Duration, request *Request, response *Response) (rn int, wn int, e error) {
	// 检查连接
	if !this.valid {
		// 失败重试
		for i := 0; i < this.reConn; i++ {
			this.conn, e = dial()
			if nil == e {
				this.valid = true
				break
			}
		}
	}
	// 发送请求
	wn, e = this.write(request, timeout)
	if nil != e {
		return
	}
	// 接收响应
	response.reset()
	rn, e = this.read(response, timeout)
	return
}

// 发送请求
func (this *conn) write(request *Request, timeout time.Duration) (wn int, e error) {
	request.cmdCount()
	// 发送请求
	if timeout > 0 {
		// 写超时
		e = this.conn.SetWriteDeadline(time.Now().Add(timeout))
		if nil != e {
			return
		}
	}
	n := 0
	if len(request.c) > 0 {
		n, e = this.conn.Write(request.c)
		wn += n
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
	n := 0
	n, e = this.readData(response, timeout)
	rn += n
	if nil != e {
		return
	}
	// 消息类型
	switch response.b[response.i[len(response.i)-1]] {
	case '+', '-', ':':
		// 这些消息都是单行
		for {
			// 是否已经读完一行
			n = len(response.b) - 2
			if n > 0 {
				if response.b[n] == '\r' && this.rbuf[n+1] == '\n' {
					response.i = append(response.i, n+2)
					break
				}
			}
			// 继续读
			n, e = this.readData(response, timeout)
			rn += n
			if nil != e {
				return
			}
		}
	case '$':
		// 这个消息要读两行
		i := response.i[len(response.i)-1] + 1
		for {
			for ; i < len(response.b); i++ {
				if response.b[i] == '\n' && response.b[i-1] == '\r' {
					response.i = append(response.i, i+1)
				}
			}
			if len(response.i) > 2 {
				break
			}
			// 继续读
			n, e = this.readData(response, timeout)
			rn += n
			if nil != e {
				return
			}
		}
	case '*':
		// 先读完数组长度行
		count := -1
	CountLoop:
		for {
			for i := 0; i < len(response.b); i++ {
				if response.b[i] == '\n' && response.b[i-1] == '\r' {
					count = parseInt(response.b[1:i-2])
					i++
					response.i = append(response.i, i)
					break CountLoop
				}
			}
			// 继续读
			n, e = this.readData(response, timeout)
			rn += n
			if nil != e {
				return
			}
		}
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

// 关闭这个连接
// 在Redis.Close函数中调用
func (this *conn) Close() error {
	if this.valid {
		this.valid = false
		return this.conn.Close()
	}
	return nil
}

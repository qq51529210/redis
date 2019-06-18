package redis

import (
	"errors"
	"net"
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
func (this *conn) Cmd(dial DialFunc, request, response *Message) (rn int, wn int, e error) {
	// 检查连接
	// 当前的连接无效
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
	wn, e = this.conn.Write(request.buf.Bytes())
	if nil != e {
		return
	}
	// 读取响应
	response.buf.Reset()
	// 先读取消息类型
	n, e := readLine(this.conn, this.rbuf, &response.buf, 1)
	rn += n
	if nil != e {
		return
	}
	buf := response.buf.Bytes()
	// 消息类型
	switch buf[0] {
	case '+', '-', ':', '$':
		// 这些消息都是单行
	case '*':
		// 继续读完数组消息
		n, e := readLine(this.conn, this.rbuf, &response.buf, parseInt(buf[1:response.buf.Len()-2]))
		rn += n
		if nil != e {
			return
		}
	default:
		e = errors.New("invalid cmd type from redis server")
	}
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

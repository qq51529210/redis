package redis

import (
	"errors"
	"net"
)

// 简单字符串，'+' + s + '\r' + '\n'
// 例，OK，"+OK\r\n"
// 一般这是服务端返回的
//
// 错误，'-' + s + '\r' + '\n'
// 例，error message，"-error message\r\n"
// 一般这是服务端返回的
//
// 整数，':' + itoa(n) + '\r' + '\n'
// 例，-100，":-100\r\n"
//
// 大容量字符串，'$' + itoa(len(s)) + '\r' + '\n' + s + '\r' + '\n'
// 例，hello，"$5\r\nhello\r\n"
// 空字符串，$0\r\n\r\n
//
// 数组，'*' + itoa(len(a)) + '\r' + '\n' + 其他类型的编码
// 例，[]{1,"hello",[]{1,2}}
// "*3\r\n"
// ":1\r\n"
// "$5\r\nhello\r\n"
// "*2\r\n"
// ":1\r\n"
// ":2\r\n"

var useClosedRedis = errors.New("use closed redis client")

var endLine = []byte{'\r', '\n'}

// 返回一个底层的到redis服务端的连接
type DialFunc func() (net.Conn, error)

func maxInt(i1, i2 int) int {
	if i1 > i2 {
		return i1
	}
	return i2
}

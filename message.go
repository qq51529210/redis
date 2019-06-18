package redis

import (
	"bytes"
	"sync"
)

var (
	// 消息池
	messagePool = sync.Pool{}
	// 结尾
	endline = []byte{'\r', '\n'}
)

func init() {
	messagePool.New = func() interface{} {
		return &Message{}
	}
}

// 从消息缓存池中获取一个Message对象
func GetMessage() *Message {
	m := messagePool.Get().(*Message)
	m.buf.Reset()
	return m
}

// 把Message对象放回消息缓存池
func PutMessage(m *Message) {
	messagePool.Put(m)
}

// 消息对象
type Message struct {
	tmp [24]byte
	buf bytes.Buffer
}

// *int\r\n
// $int\r\n
// string\r\n
// $int\r\n
// string\r\n
// ...
// 写入命令消息
func (this *Message) Write(cmd ... string) {
	n := len(cmd)
	if n < 1 {
		return
	}
	this.buf.Reset()
	// 数组个数
	this.writeInt('*', len(cmd))
	// 字符串
	for i := 0; i < len(cmd); i++ {
		this.bulkStrings(cmd[i])
	}
}

// :int\r\n
// 写入整数消息
func (this *Message) Integer(n int) {
	this.writeInt(':', n)
}

// $int\r\n
// string\r\n
// 字符串格式
func (this *Message) bulkStrings(s string) {
	n := len(s)
	this.writeInt('$', n)
	this.buf.WriteString(s)
	this.buf.Write(endline)
}

// +string\r\n
// -string\r\n
// 一般是redis服务发过来的响应格式
func (this *Message) simpleStrings(c byte, s string) {
	this.buf.WriteByte(c)
	this.buf.WriteString(s)
	this.buf.Write(endline)
}

// :int\r\n
// $int\r\n
// *int\r\n
// 写入:/$/*整数
func (this *Message) writeInt(c byte, n int) {
	this.tmp[0] = c
	this.buf.Write(this.tmp[:formatInt(this.tmp[1:], n)+1])
	this.buf.Write(endline)
	this.buf.Write(this.tmp[:n+2])
}

func (this *Message) Read() (string, bool) {
	if this.buf.Len() < 1 {
		return "", false
	}

}

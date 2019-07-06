package redis

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
)

// 消息中的数据类型
type DataType int

func (this DataType) String() string {
	switch this {
	case DataTypeNil:
		return "nil"
	case DataTypeResponse:
		return "response"
	case DataTypeError:
		return "error"
	case DataTypeInteger:
		return "integer"
	case DataTypeString:
		return "string"
	case DataTypeArray:
		return "array"
	default:
		return "unknown"
	}
}

const (
	DataTypeNil      DataType = iota
	DataTypeResponse
	DataTypeError
	DataTypeInteger
	DataTypeString
	DataTypeArray
	DataTypeUnknown
)

var (
	// 消息池
	requestPool  = sync.Pool{}
	responsePool = sync.Pool{}
	messagePool  = sync.Pool{}
	// 结尾
	crlf = []byte{'\r', '\n'}
	// 缓存池
	bufferPool = sync.Pool{}
	// 错误
	ErrUnknownDataType = errors.New("unknown data type from server")
)

func init() {
	requestPool.New = func() interface{} {
		return &Request{}
	}
	responsePool.New = func() interface{} {
		return &Response{}
	}
	messagePool.New = func() interface{} {
		return &Message{}
	}
	bufferPool.New = func() interface{} {
		return make([]byte, 0)
	}
}

func GetRequest() *Request {
	//r := requestPool.Get().(*Request)
	//r.Reset()
	//return r
	return requestPool.Get().(*Request)
}

func PutRequest(m *Request) {
	requestPool.Put(m)
}

func GetResponse() *Response {
	return responsePool.Get().(*Response)
}

func PutResponse(m *Response) {
	requestPool.Put(m)
}

func GetMessage() *Message {
	return messagePool.Get().(*Message)
}

func PutMessage(m *Message) {
	messagePool.Put(m)
}

// 请求对象
type Request struct {
	b []byte   // 缓存
	c [24]byte // 顶级的命令的个数缓存
	n int      // 写入的个数
}

// 重置缓存
func (this *Request) Reset() *Request {
	this.b = this.b[:0]
	this.n = 0
	return this
}

// 写入字符串($)，或者是其他二进制数据
func (this *Request) String(s string) *Request {
	// 字符串长度
	this.string(s)
	this.n++
	return this
}

func (this *Request) string(s string) {
	// 字符串长度
	this.length('$', len(s))
	// 字符串
	this.b = append(this.b, s...)
	// crlf
	this.b = append(this.b, crlf...)
}

// 写入二进制数据($)，其实也会转换为字符串
func (this *Request) Bytes(b []byte) *Request {
	this.bytes(b)
	this.n++
	return this
}

func (this *Request) bytes(b []byte) *Request {
	// 字符串长度
	this.length('$', len(b))
	// 二进制
	this.b = append(this.b, b...)
	// crlf
	this.b = append(this.b, crlf...)
	return this
}

// 写入整数(:)
// 通信协议上这样写，redis服务却又不接受整数值，必须转成string
// ERR Protocol error: expected '$', got ':', error
func (this *Request) Integer(n int64) *Request {
	this.integer(n)
	this.n++
	return this
}

func (this *Request) integer(n int64) *Request {
	m := formatInt(this.c[0:], n)
	this.length('$', m)
	this.b = append(this.b, this.c[:m]...)
	this.b = append(this.b, crlf...)
	return this
}

// 依次写入标记($)/(:)/(*)，整数(n)，crlf
func (this *Request) length(c byte, n int) {
	// 标记，:/$/*
	this.b = append(this.b, c)
	// 负数
	if n < 0 {
		this.b = append(this.b, '-')
		n = 0 - n
	}
	// 写入数组
	i1 := len(this.b)
	for n > 0 {
		this.b = append(this.b, byte('0'+n%10))
		n /= 10
	}
	// 反转
	i2 := len(this.b) - 1
	for i1 < i2 {
		c = this.b[i1]
		this.b[i1] = this.b[i2]
		this.b[i2] = c
		i2--
		i1++
	}
	// 换行crlf
	this.b = append(this.b, crlf...)
}

// 写入整数数组(*)
// redis服务不接受数组值
// ERR Protocol error: expected '$', got '*', error
func (this *Request) ArrayInt(a []int64) *Request {
	this.length('*', len(a))
	for i := 0; i < len(a); i++ {
		this.integer(a[i])
	}
	this.n++
	return this
}

// 写入字符串数组(*)
// redis服务不接受数组值
// ERR Protocol error: expected '$', got '*', error
func (this *Request) ArrayString(a []string) *Request {
	this.length('*', len(a))
	for i := 0; i < len(a); i++ {
		this.string(a[i])
	}
	this.n++
	return this
}

// 写入字符串数组(*)
// redis服务不接受数组值
// ERR Protocol error: expected '$', got '*', error
func (this *Request) ArrayBytes(a [][]byte) *Request {
	this.length('*', len(a))
	for i := 0; i < len(a); i++ {
		this.bytes(a[i])
	}
	this.n++
	return this
}

// 写入命令，会清除掉上一次的命令
// 如果想写入整数值，需要转换
func (this *Request) Write(cmd ... string) *Request {
	n := len(cmd)
	if n < 1 {
		return this
	}
	// 字符串
	for i := 0; i < n; i++ {
		this.String(cmd[i])
	}
	return this
}

// 写入字符串(+)，或者是错误(-)
// 一般是redis服务发过来的响应格式
// 就不导出了
func (this *Request) simpleStrings(c byte, s string) {
	// 标记，+/-
	this.b = append(this.b, c)
	// 字符串
	this.b = append(this.b, s...)
	// 换行crlf
	this.b = append(this.b, crlf...)
}

// 消息对象
type Response struct {
	b []byte // 数据缓存
	i []int  // 下一行的起始索引
	n int    // 为了i不重新分配内存
}

// 重置缓存
func (this *Response) reset() {
	this.b = this.b[:0]
	this.i = this.i[:0]
	this.i = append(this.i, 0)
	this.n = 0
}

// 返回下一个值(字符串表示)，和值的类型
func (this *Response) Read() (string, DataType) {
	if this.n == len(this.i)-1 {
		return "", DataTypeNil
	}
	i1 := this.i[this.n]
	this.n++
	i2 := this.i[this.n]
	switch this.b[i1] {
	case '+':
		return string(this.b[i1+1:i2-2]), DataTypeResponse
	case '-':
		return string(this.b[i1+1:i2-2]), DataTypeError
	case ':':
		return string(this.b[i1+1:i2-2]), DataTypeInteger
	case '$':
		// 下一个
		if this.n == len(this.i)-1 {
			return "", DataTypeNil
		}
		i1 := this.i[this.n]
		this.n++
		i2 := this.i[this.n]
		return string(this.b[i1:i2-2]), DataTypeString
	case '*':
		return string(this.b[i1+1:i2-2]), DataTypeArray
	default:
		return "", DataTypeUnknown
	}
}

// 返回下一个数据类型，并将数据写到writer中
func (this *Response) ReadTo(w io.Writer) (DataType, error) {
	if this.n == len(this.i)-1 {
		return DataTypeNil, nil
	}
	i1 := this.i[this.n]
	this.n++
	i2 := this.i[this.n]
	switch this.b[i1] {
	case '+':
		_, e := w.Write(this.b[i1+1:i2-2])
		return DataTypeResponse, e
	case '-':
		_, e := w.Write(this.b[i1+1:i2-2])
		return DataTypeError, e
	case ':':
		_, e := w.Write(this.b[i1+1:i2-2])
		return DataTypeInteger, e
	case '$':
		// 下一个
		if this.n == len(this.i)-1 {
			return DataTypeNil, nil
		}
		i1 := this.i[this.n]
		this.n++
		i2 := this.i[this.n]
		_, e := w.Write(this.b[i1:i2-2])
		return DataTypeString, e
	case '*':
		_, e := w.Write(this.b[i1+1:i2-2])
		return DataTypeArray, e
	default:
		return DataTypeUnknown, ErrUnknownDataType
	}
}

// 查找第一个crlf并写入标记
func (this *Response) crlf(i int) int {
	for ; i < len(this.b); i++ {
		if this.b[i] == '\n' && this.b[i-1] == '\r' {
			this.i = append(this.i, i+1)
			return i
		}
	}
	return -1
}

type Message struct {
	Request
	Response
	bytes.Buffer
	strings.Builder
}

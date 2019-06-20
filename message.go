package redis

import (
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
	// 结尾
	crlf = []byte{'\r', '\n'}
	// 缓存池
	bufferPool = sync.Pool{}
)

func init() {
	requestPool.New = func() interface{} {
		return &Request{}
	}
	responsePool.New = func() interface{} {
		return &Response{}
	}
	bufferPool.New = func() interface{} {
		return make([]byte, 0)
	}
}

func GetRequest() *Request {
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

// 请求对象
type Request struct {
	b []byte // 缓存
	c []byte // 顶级的命令的个数缓存
	n int    // 写入的个数
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
	this.b = this.integer('$', len(s), this.b)
	// 字符串
	this.b = append(this.b, s...)
	// 换行crlf
	this.b = append(this.b, crlf...)
	this.n++
	return this
}

// 写入二进制数据($)，其实也会转换为字符串
func (this *Request) Bytes(b []byte) *Request {
	// 字符串长度
	this.b = this.integer('$', len(b), this.b)
	// 字符串
	this.b = append(this.b, b...)
	// 换行crlf
	this.b = append(this.b, crlf...)
	this.n++
	return this
}

// 写入整数(:)
func (this *Request) Integer(n int) *Request {
	this.b = this.integer(':', n, this.b)
	this.n++
	return this
}

// 写入整数数组(*)
func (this *Request) ArrayInt(a []int) *Request {
	this.b = this.integer('*', len(a), this.b)
	for i := 0; i < len(a); i++ {
		this.b = this.integer(':', a[i], this.b)
	}
	this.n++
	return this
}

// 写入字符串数组(*)
func (this *Request) ArrayString(a []string) *Request {
	this.b = this.integer('*', len(a), this.b)
	for i := 0; i < len(a); i++ {
		this.String(a[i])
	}
	this.n++
	return this
}

// 写入字符串数组(*)
func (this *Request) ArrayBytes(a [][]byte) *Request {
	this.b = this.integer('*', len(a), this.b)
	for i := 0; i < len(a); i++ {
		this.Bytes(a[i])
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
	// 重置
	this.b = this.b[:0]
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

// 依次写入标记($)/(:)/(*)，整数(n)，crlf
func (this *Request) integer(c byte, n int, b []byte) []byte {
	// 标记，:/$/*
	b = append(b, c)
	// 负数
	if n < 0 {
		b = append(b, '-')
		n = 0 - n
	}
	// 写入数组
	i1 := len(b)
	for n > 0 {
		b = append(b, byte('0'+n%10))
		n /= 10
	}
	// 反转
	i2 := len(b) - 1
	for i1 < i2 {
		c = b[i1]
		b[i1] = b[i2]
		b[i2] = c
		i2--
		i1++
	}
	// 换行crlf
	b = append(b, crlf...)
	return b
}

// 写入顶级的命令个数
func (this *Request) cmdCount() {
	this.c = this.c[:0]
	if this.n > 1 {
		this.c = this.integer('*', this.n, this.c)
	}
}

// 消息对象
type Response struct {
	b []byte // 数据缓存
	i []int  // 下一行的起始索引
}

// 重置缓存
func (this *Response) reset() {
	this.b = this.b[:0]
	this.i = this.i[:0]
	this.i = append(this.i, 0)
}

// 返回下一个值(字符串表示)，和值的类型
func (this *Response) Read() (string, DataType) {
	// 没有结果集
	if len(this.i) < 2 {
		return "", DataTypeNil
	}
	i1 := this.i[0]
	this.i = this.i[1:]
	i2 := this.i[0]
	switch this.b[i1] {
	case '+':
		return string(this.b[i1+1:i2-2]), DataTypeResponse
	case '-':
		return string(this.b[i1+1:i2-2]), DataTypeError
	case ':':
		return string(this.b[i1+1:i2-2]), DataTypeInteger
	case '$':
		// 下一个
		if len(this.i) < 2 {
			return "", DataTypeNil
		}
		i1 = this.i[0]
		this.i = this.i[1:]
		i2 := this.i[0]
		return string(this.b[i1:i2-2]), DataTypeString
	case '*':
		return string(this.b[i1+1:i2-2]), DataTypeArray
	default:
		return "", DataTypeUnknown
	}
}

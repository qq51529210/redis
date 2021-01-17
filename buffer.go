package redis

import (
	"encoding/json"
	"strconv"
)

// 缓存池
var (
	endLine = []byte{'\r', '\n'}
)

// buf[...i1...i2...len...]
type buffer struct {
	fmt []byte // 格式化数组的缓存，减少内存分配
	buf []byte // 请求和响应的数据缓存
	i1  int    // 新的一行数据的起始
	i2  int    // 下一个解析索引
	len int    // res数据的大小
}

func (b *buffer) Value(a interface{}) {
	switch v := a.(type) {
	case int:
		b.Int(int64(v))
	case uint:
		b.Int(int64(v))
	case int8:
		b.Int(int64(v))
	case uint8:
		b.Int(int64(v))
	case int16:
		b.Int(int64(v))
	case uint16:
		b.Int(int64(v))
	case int32:
		b.Int(int64(v))
	case uint32:
		b.Int(int64(v))
	case int64:
		b.Int(v)
	case uint64:
		b.Int(int64(v))
	case float32:
		b.Float(float64(v))
	case float64:
		b.Float(v)
	case string:
		b.String(v)
	case []byte:
		b.Bytes(v)
	case []interface{}:
		b.Array(v)
	case nil:
		b.Nil()
	default:
		b.Json(v)
	}
}

// 简单字符串，'+' + s + '\r' + '\n'
// 例，OK，"+OK\r\n"
// 一般这是服务端返回的
func (b *buffer) SimpleString(s string) {
	// '+'
	b.buf = append(b.buf, '+')
	// str
	b.buf = append(b.buf, s...)
	// \r\n
	b.buf = append(b.buf, endLine...)
}

// 错误，'-' + s + '\r' + '\n'
// 例，error message，"-error message\r\n"
// 一般这是服务端返回的
func (b *buffer) Error(s string) {
	// '-'
	b.buf = append(b.buf, '-')
	// str
	b.buf = append(b.buf, s...)
	// \r\n
	b.buf = append(b.buf, endLine...)
}

// 文档这么写
// 整数，':' + string(n) + '\r' + '\n'
// 例，-100，":-100\r\n"
//
// 但是然并卵，必须用字符串来存储
func (b *buffer) Int(n int64) {
	//// :
	//b.buf = append(b.buf, ':')
	//// string(n)
	//b.buf = strconv.AppendInt(b.buf, n, 10)
	//// \r\n
	//b.buf = append(b.buf, endLine...)
	b.fmt = b.fmt[:0]
	b.fmt = strconv.AppendInt(b.fmt, n, 10)
	b.Bytes(b.fmt)
}

// 使用字符串来存储
func (b *buffer) Float(v float64) {
	b.fmt = b.fmt[:0]
	b.fmt = strconv.AppendFloat(b.fmt, v, 'f', -1, 64)
	b.Bytes(b.fmt)
}

// 大容量字符串，'$' + itoa(len(s)) + '\r' + '\n' + s + '\r' + '\n'
// 例，hello，"$5\r\nhello\r\n"
// 空字符串，$0\r\n\r\n
func (b *buffer) String(v string) {
	// '$'
	b.buf = append(b.buf, '$')
	// string(length)
	b.buf = strconv.AppendInt(b.buf, int64(len(v)), 10)
	// \r\n
	b.buf = append(b.buf, endLine...)
	// str
	b.buf = append(b.buf, v...)
	// \r\n
	b.buf = append(b.buf, endLine...)
}

// 使用字符串来存储
func (b *buffer) Bytes(v []byte) {
	// '$'
	b.buf = append(b.buf, '$')
	// string(length)
	b.buf = strconv.AppendInt(b.buf, int64(len(v)), 10)
	// \r\n
	b.buf = append(b.buf, endLine...)
	// s
	b.buf = append(b.buf, v...)
	// \r\n
	b.buf = append(b.buf, endLine...)
}

// 数组，'*' + itoa(len(a)) + '\r' + '\n' + 其他类型的编码
// 例，[]{1,"hello",[]{1,2}}
// "*3\r\n"
// ":1\r\n"
// "$5\r\nhello\r\n"
// "*2\r\n"
// ":1\r\n"
// ":2\r\n"
func (b *buffer) Array(v []interface{}) {
	// '*'
	b.buf = append(b.buf, '*')
	// string(length)
	b.buf = strconv.AppendInt(b.buf, int64(len(v)), 10)
	// \r\n
	b.buf = append(b.buf, endLine...)
	// values
	for _, a := range v {
		b.Value(a)
	}
}

func (b *buffer) Json(v interface{}) {
	d, _ := json.Marshal(v)
	b.Bytes(d)
}

func (b *buffer) CmdCount(n int64) {
	b.buf = b.buf[:0]
	// '*'
	b.buf = append(b.buf, '*')
	// count
	b.buf = strconv.AppendInt(b.buf, n, 10)
	// \r\n
	b.buf = append(b.buf, endLine...)
}

func (b *buffer) Nil() {
	// -1
	b.buf = strconv.AppendInt(b.buf, -1, 10)
	// \r\n
	b.buf = append(b.buf, endLine...)
}

func (b *buffer) ReadLine() ([]byte, bool) {
	if !b.indexLine() {
		// 缓存不够，扩容
		if b.i2 == len(b.buf) {
			b.resize()
		}
		return nil, false
	}
	i1 := b.i1
	i2 := b.i2
	b.i1 = b.i2
	if b.i1 == b.len {
		b.i1 = 0
		b.i2 = 0
		b.len = 0
	}
	return b.buf[i1:i2], true
}

func (b *buffer) indexLine() bool {
	for ; b.i2 < b.len; b.i2++ {
		if b.buf[b.i2] == '\n' && b.buf[b.i2-1] == '\r' {
			b.i2++
			return true
		}
	}
	return false
}

func (b *buffer) resize() {
	newBuf := make([]byte, len(b.buf)*2)
	copy(newBuf, b.buf)
	b.buf = newBuf
}

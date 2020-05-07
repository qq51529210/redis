package redis

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
)

// 缓存池
var _cmd sync.Pool

func init() {
	_cmd.New = func() interface{} {
		return new(Cmd)
	}
}

// 从缓存池获取一个Cmd对象
func getCmd() *Cmd {
	cmd := _cmd.Get().(*Cmd)
	return cmd
}

// 把Cmd对象放回缓存池
func putCmd(cmd *Cmd) {
	// 重置缓存
	cmd.Reset()
	_cmd.Put(cmd)
}

// 表示一条redis命令
type Cmd struct {
	buf [256]byte // 读取响应数据/格式化的缓存
	req []byte    // 请求数据
	cot int       // 当前命令的个数
	res []byte    // 未解析的响应数据
	idx int       // 未解析的响应数据开始下标
	fmt []byte    // 格式化数组的缓存，减少内存分配
}

// 重置请求缓存，除非自己保存，所有Client.NewCmd()自动调用了
func (this *Cmd) Reset() {
	this.req = this.req[:0]
	this.cot = 0
}

// 编码简单字符串
func (this *Cmd) SimpleString(s string) {
	this.cot++
	// '+'
	this.req = append(this.req, '+')
	// s
	this.req = append(this.req, s...)
	// \r\n
	this.req = append(this.req, endLine...)
}

// 编码错误
func (this *Cmd) Error(s string) {
	this.cot++
	// '-'
	this.req = append(this.req, '-')
	// s
	this.req = append(this.req, s...)
	// \r\n
	this.req = append(this.req, endLine...)
}

// 格式化整数，作为字符串处理，否则服务端会报错
// 定义这个':'协议，但不让客户端用，很奇怪
func (this *Cmd) Int(n int64) {
	this.fmt = this.fmt[:0]
	this.fmt = strconv.AppendInt(this.fmt, n, 10)
	this.Bytes(this.fmt)
	//this.cot++
	//// ':'
	//this.req = append(this.req, ':')
	//// n
	//this.req = strconv.AppendInt(this.req, n, 10)
	//// \r\n
	//this.req = append(this.req, endLine...)
}

// 编码字符串
func (this *Cmd) String(s string) {
	this.cot++
	// '$'
	this.req = append(this.req, '$')
	// length
	this.req = strconv.AppendInt(this.req, int64(len(s)), 10)
	// \r\n
	this.req = append(this.req, endLine...)
	// b
	this.req = append(this.req, s...)
	// \r\n
	this.req = append(this.req, endLine...)
}

// 编码二进制数组，作为字符串处理
func (this *Cmd) Bytes(b []byte) {
	this.cot++
	// '$'
	this.req = append(this.req, '$')
	// length
	this.req = strconv.AppendInt(this.req, int64(len(b)), 10)
	// \r\n
	this.req = append(this.req, endLine...)
	// b
	this.req = append(this.req, b...)
	// \r\n
	this.req = append(this.req, endLine...)
}

// 格式化浮点数，作为字符串处理
func (this *Cmd) Float(n float64) {
	this.fmt = this.fmt[:0]
	this.fmt = strconv.AppendFloat(this.fmt, n, 'f', -1, 64)
	this.Bytes(this.fmt)
	//this.cot++
	//// '$'
	//this.req = append(this.req, '$')
	//strconv.app
}

// 编码一个对象
// switch v.(type)
// case intxx,uintxx: Int()
// case string: String()
// case Floatxx: Float()
// case []byte: Bytes()
// default: Json
func (this *Cmd) Value(v interface{}) error {
	this.cot++
	return this.value(v)
}

func (this *Cmd) value(v interface{}) (err error) {
	switch v.(type) {
	case int8:
		this.Int(int64(v.(int8)))
	case uint8:
		this.Int(int64(v.(uint8)))
	case int16:
		this.Int(int64(v.(int16)))
	case uint16:
		this.Int(int64(v.(uint16)))
	case int32:
		this.Int(int64(v.(int32)))
	case uint32:
		this.Int(int64(v.(uint32)))
	case int64:
		this.Int(v.(int64))
	case uint64:
		this.Int(int64(v.(uint64)))
	case int:
		this.Int(int64(v.(int)))
	case uint:
		this.Int(int64(v.(uint)))
	case float32:
		this.Float(float64(v.(float32)))
	case float64:
		this.Float(v.(float64))
	case string:
		this.String(v.(string))
	case []byte:
		this.Bytes(v.([]byte))
	case []interface{}:
		err = this.Array(v.([]interface{}))
	default:
		err = this.Json(v)
	}
	return
}

// 编码数组
func (this *Cmd) Array(a []interface{}) (err error) {
	this.cot++
	// '*'
	this.req = append(this.req, '*')
	// count
	this.req = strconv.AppendInt(this.req, int64(len(a)), 10)
	// \r\n
	this.req = append(this.req, endLine...)
	// item
	for i := 0; i < len(a); i++ {
		err = this.value(a[i])
		if err != nil {
			break
		}
	}
	return
}

// 格式化json字符串，作为字符串处理
func (this *Cmd) Json(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	this.Bytes(data)
	return nil
}

// 返回格式化的请求缓存
func (this *Cmd) Cmd() []byte {
	return this.req
}

func (this *Cmd) WriteTo(writer io.Writer) (int64, error) {
	this.fmt = this.fmt[:0]
	// '*'
	this.fmt = append(this.fmt, '*')
	// count
	this.fmt = strconv.AppendInt(this.fmt, int64(this.cot), 10)
	// \r\n
	this.fmt = append(this.fmt, endLine...)
	// 先写命令单词的个数
	n, err := writer.Write(this.fmt)
	if err != nil {
		return 0, err
	}
	// 在写命令
	m, err := writer.Write(this.req)
	return int64(n + m), err
}

// 读取一个数据
func (this *Cmd) ReadValue(reader io.Reader) (interface{}, error) {
	// 先读一行
	line, err := this.readLine(reader)
	if err != nil {
		return nil, err
	}
	// 判断数据类型
	switch line[0] {
	case '+': // 简单字符串
		return string(line[1 : len(line)-2]), nil
	case '-': // 错误
		return errors.New(string(line[1 : len(line)-2])), nil
	case ':': // 整数
		return parseInt(line[1 : len(line)-2])
	case '$': // 字符串
		length, err := parseInt(line[1 : len(line)-2])
		if err != nil {
			return nil, err
		}
		// 再读n+2个字节
		line, err = this.readN(reader, length+2)
		if err != nil {
			return nil, err
		}
		return string(line[:len(line)-2]), nil
	case '*': // 数组
		count, err := parseInt(line[1 : len(line)-2])
		if err != nil {
			return nil, err
		}
		var values []interface{}
		for i := 0; i < int(count); i++ {
			value, err := this.ReadValue(reader)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("invalid data type %d from server", this.buf[0])
	}
}

// 从reader，或者this.res中，读取1行...\r\n
func (this *Cmd) readLine(reader io.Reader) ([]byte, error) {
	var n, i int
	var err error
	// 是否有未处理的数据
	if len(this.res) > 0 {
		if len(this.res) == this.idx {
			// 没有未处理的数据，重置
			this.res = this.res[:0]
			this.idx = 0
		} else {
			// 有未处理的数据，检查有没有完整的一行
			i = indexEndLine(this.res[this.idx:])
			if i >= 0 {
				// 有完整的一行
				n = this.idx
				this.idx += i
				return this.res[n:this.idx], nil
			}
		}
	}
	// 开始
	for {
		// 读数据
		n, err = reader.Read(this.buf[:])
		if err != nil {
			return nil, err
		}
		i = indexEndLine(this.buf[:n])
		if i >= 0 {
			if this.idx > 0 {
				// 接着上一次没有读完的数据
				this.res = append(this.res, this.buf[:n]...)
				this.idx += i
				return this.res[:this.idx], nil
			}
			// 不拷贝到res
			if n > i {
				// 有多余的数据
				this.res = append(this.res, this.buf[i:n]...)
			}
			// 返回，带\r\n
			return this.buf[:i], nil
		}
		// 没有，继续
		this.res = append(this.res, this.buf[:n]...)
		this.idx = len(this.res)
	}
}

// 从reader，或者this.res中，读取n个字节
func (this *Cmd) readN(reader io.Reader, m int64) ([]byte, error) {
	var n int
	var err error
	// 是否有未处理的数据
	if len(this.res) > 0 {
		if len(this.res) == this.idx {
			// 没有未处理的数据，重置
			this.res = this.res[:0]
			this.idx = 0
		} else {
			// 有未处理的数据，检查是否够m个字节
			if int64(len(this.res[this.idx:])) >= m {
				// 有m个字节
				n := this.idx
				this.idx += int(m)
				return this.res[n:this.idx], nil
			}
		}
	}
	// 开始
	for {
		// 读数据
		n, err = reader.Read(this.buf[:])
		if err != nil {
			return nil, err
		}
		if this.idx > 0 {
			// 接着上一次没有读完的数据
			this.res = append(this.res, this.buf[:n]...)
			if int64(len(this.res[this.idx:])) >= m {
				// 有m个字节
				n := this.idx
				this.idx += int(m)
				return this.res[n:this.idx], nil
			}
		} else {
			// 上一次没有数据
			if int64(n) == m {
				// 正好
				return this.buf[:n], nil
			} else if int64(n) > m {
				// 多余的数据，保存
				this.res = append(this.res, this.buf[m:n]...)
				return this.buf[:m], nil
			}
		}
		// 不够，继续
	}
}

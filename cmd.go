package redis

import (
	"encoding/json"
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
	cmd int       // 当前命令的个数
	res []byte    // 未解析的响应数据
	idx int       // 未解析的响应数据开始下标
	fmt []byte    // 格式化数组的缓存，减少内存分配
}

// 重置请求缓存，除非自己保存，所有Client.NewCmd()自动调用了
func (c *Cmd) Reset() {
	c.req = c.req[:0]
	c.cmd = 0
}

func (c *Cmd) appendBytes(b []byte) {
	// b
	c.req = append(c.req, b...)
	// \r\n
	c.req = append(c.req, endLine...)
}

func (c *Cmd) appendString(s string) {
	// s
	c.req = append(c.req, s...)
	// \r\n
	c.req = append(c.req, endLine...)
}

func (c *Cmd) addLength(b byte, n int64) {
	// b
	c.req = append(c.req, b)
	// length
	c.req = strconv.AppendInt(c.req, n, 10)
	// \r\n
	c.req = append(c.req, endLine...)
}

func (c *Cmd) addString(s string) {
	n := int64(len(s))
	// $n\r\n
	c.addLength('$', n)
	// b\r\n
	c.appendString(s)
}

func (c *Cmd) addBytes(b []byte) {
	n := int64(len(b))
	// $n\r\n
	c.addLength('$', n)
	// b\r\n
	c.appendBytes(b)
}

func (c *Cmd) addInt(n int64) {
	c.fmt = c.fmt[:0]
	c.fmt = strconv.AppendInt(c.fmt, n, 10)
	c.addBytes(c.fmt)
}

func (c *Cmd) addFloat(n float64) {
	c.fmt = c.fmt[:0]
	c.fmt = strconv.AppendFloat(c.fmt, n, 'f', -1, 64)
	c.addBytes(c.fmt)
}

func (c *Cmd) addJson(v interface{}) error {
	d, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.addBytes(d)
	return nil
}

func (c *Cmd) addArray(a []interface{}) (err error) {
	n := int64(len(a))
	// *n\r\n
	c.addLength('*', n)
	if n < 1 {
		return
	}
	// a
	for i := 0; i < len(a); i++ {
		err = c.addValue(a[i])
		if err != nil {
			break
		}
	}
	return
}

// +OK\r\n
func (c *Cmd) AddSimpleString(s string) {
	c.cmd++
	c.req = append(c.req, '+')
	c.appendString(s)
}

// -Error\r\n
func (c *Cmd) AddError(s string) {
	c.cmd++
	c.req = append(c.req, '-')
	c.appendString(s)
}

// :1\r\n
// 这里先转换成字符串，否则服务端会报错，定义这个':'协议，但不让客户端用，很奇怪
func (c *Cmd) AddInt(n int64) {
	c.cmd++
	c.addInt(n)
}

// $5\r\nhello\r\n，$0\r\n表示空字符串，$-1\r\n表示null
func (c *Cmd) AddString(s string) {
	c.cmd++
	c.addString(s)
}

// 作为字符串处理
func (c *Cmd) AddBytes(b []byte) {
	c.cmd++
	c.addBytes(b)
}

// 作为字符串处理
func (c *Cmd) AddFloat(n float64) {
	c.cmd++
	c.addFloat(n)
}

// 编码一个对象
// switch v.(type)
// case intxx,uintxx: AddInt()
// case string: AddString()
// case floatxx: AddFloat()
// case []byte: AddBytes()
// case nil: AddNil()
// default: AddJson()
func (c *Cmd) AddValue(v interface{}) error {
	c.cmd++
	return c.addValue(v)
}

func (c *Cmd) addValue(v interface{}) (err error) {
	if v == nil {
		c.addLength('$', -1)
		return
	}
	switch v.(type) {
	case int8:
		c.addInt(int64(v.(int8)))
	case uint8:
		c.addInt(int64(v.(uint8)))
	case int16:
		c.addInt(int64(v.(int16)))
	case uint16:
		c.addInt(int64(v.(uint16)))
	case int32:
		c.addInt(int64(v.(int32)))
	case uint32:
		c.addInt(int64(v.(uint32)))
	case int64:
		c.addInt(v.(int64))
	case uint64:
		c.addInt(int64(v.(uint64)))
	case int:
		c.addInt(int64(v.(int)))
	case uint:
		c.addInt(int64(v.(uint)))
	case float32:
		c.addFloat(float64(v.(float32)))
	case float64:
		c.addFloat(v.(float64))
	case string:
		c.addString(v.(string))
	case []byte:
		c.addBytes(v.([]byte))
	case []interface{}:
		err = c.addArray(v.([]interface{}))
	default:
		err = c.addJson(v)
	}
	return
}

// *n\r\na\r\n[...]
func (c *Cmd) AddArray(a []interface{}) (err error) {
	c.cmd++
	return c.addArray(a)
}

// 格式化json字符串，作为字符串处理
func (c *Cmd) AddJson(v interface{}) error {
	c.cmd++
	return c.addJson(v)
}

// $-1\r\n
func (c *Cmd) AddNil() {
	c.cmd++
	c.addLength('$', -1)
}

// 返回格式化的请求缓存
func (c *Cmd) Data() []byte {
	return c.req
}

// io.WriteTo
func (c *Cmd) WriteTo(w io.Writer) (int64, error) {
	c.fmt = c.fmt[:0]
	// '*'
	c.fmt = append(c.fmt, '*')
	// count
	c.fmt = strconv.AppendInt(c.fmt, int64(c.cmd), 10)
	// \r\n
	c.fmt = append(c.fmt, endLine...)
	// 先写命令单词的个数
	n, err := w.Write(c.fmt)
	if err != nil {
		return 0, err
	}
	// 在写命令
	m, err := w.Write(c.req)
	return int64(n + m), err
}

// 读取一个数据
func (c *Cmd) readValue(r io.Reader) (interface{}, error) {
	// 先读一行
	line, err := c.readLine(r)
	if err != nil {
		return nil, err
	}
	// 判断数据类型
	switch line[0] {
	case '+', '-': // 简单字符串，或者错误
		return string(line[1 : len(line)-2]), nil
	case ':': // 整数
		return parseInt(line[1 : len(line)-2])
	case '$': // 字符串
		n, err := parseInt(line[1 : len(line)-2])
		if err != nil {
			return nil, err
		}
		if n < 1 {
			return "", nil
		}
		// 再读n+2个字节
		line, err = c.readN(r, n+2)
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
			value, err := c.readValue(r)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("invalid data type %d from server", c.buf[0])
	}
}

// 从r，或者c.res中，读取1行...\r\n
func (c *Cmd) readLine(r io.Reader) ([]byte, error) {
	var n, i int
	var err error
	// 是否有未处理的数据
	if len(c.res) > 0 {
		if len(c.res) == c.idx {
			// 没有未处理的数据，重置
			c.res = c.res[:0]
			c.idx = 0
		} else {
			// 有未处理的数据，检查有没有完整的一行
			i = indexEndLine(c.res[c.idx:])
			if i >= 0 {
				// 有完整的一行
				n = c.idx
				c.idx += i
				return c.res[n:c.idx], nil
			}
		}
	}
	// 开始
	for {
		// 读数据
		n, err = r.Read(c.buf[:])
		if err != nil {
			return nil, err
		}
		i = indexEndLine(c.buf[:n])
		if i > 0 {
			// 上一次有没有读完的数据
			if c.idx > 0 {
				c.res = append(c.res, c.buf[:n]...)
				c.idx += i
				return c.res[:c.idx], nil
			}
			// 不拷贝，直接返回
			if i == n {
				return c.buf[:n], nil
			}
			c.res = append(c.res, c.buf[i:n]...)
			return c.buf[:i], nil
		}
		// 没有，继续
		c.res = append(c.res, c.buf[:n]...)
		c.idx = len(c.res)
	}
}

// 从r，或者c.res中，读取n个字节
func (c *Cmd) readN(r io.Reader, m int64) ([]byte, error) {
	var n int
	var err error
	// 是否有未处理的数据
	if len(c.res) > 0 {
		if len(c.res) == c.idx {
			// 没有未处理的数据，重置
			c.res = c.res[:0]
			c.idx = 0
		} else {
			// 有未处理的数据，检查是否够m个字节
			if int64(len(c.res[c.idx:])) >= m {
				// 有m个字节
				n = c.idx
				c.idx += int(m)
				return c.res[n:c.idx], nil
			}
		}
	}
	// 开始
	for {
		// 读数据
		n, err = r.Read(c.buf[:])
		if err != nil {
			return nil, err
		}
		// 没有数据
		if len(c.res) < 1 {
			if int64(n) == m {
				// 正好，直接返回，不拷贝
				return c.buf[:n], nil
			} else if int64(n) > m {
				// 多余的数据，保存
				c.res = append(c.res, c.buf[m:n]...)
				return c.buf[:m], nil
			}
			// 不够，继续
		}
		c.res = append(c.res, c.buf[:n]...)
		if int64(len(c.res[c.idx:])) >= m {
			// 有m个字节
			n = c.idx
			c.idx += int(m)
			return c.res[n:c.idx], nil
		}
	}
}

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

// 编码简单字符串
func (c *Cmd) AddSimpleString(s string) {
	c.cmd++
	// '+'
	c.req = append(c.req, '+')
	// s
	c.req = append(c.req, s...)
	// \r\n
	c.req = append(c.req, endLine...)
}

// 编码错误
func (c *Cmd) AddError(s string) {
	c.cmd++
	// '-'
	c.req = append(c.req, '-')
	// s
	c.req = append(c.req, s...)
	// \r\n
	c.req = append(c.req, endLine...)
}

// 格式化整数，作为字符串处理，否则服务端会报错
// 定义这个':'协议，但不让客户端用，很奇怪
func (c *Cmd) AddInt(n int64) {
	c.fmt = c.fmt[:0]
	c.fmt = strconv.AppendInt(c.fmt, n, 10)
	c.AddBytes(c.fmt)
}

// 编码字符串
func (c *Cmd) AddString(s string) {
	c.cmd++
	// '$'
	c.req = append(c.req, '$')
	// length
	n := int64(len(s))
	c.req = strconv.AppendInt(c.req, n, 10)
	// \r\n
	c.req = append(c.req, endLine...)
	if n < 1 {
		return
	}
	// s
	c.req = append(c.req, s...)
	// \r\n
	c.req = append(c.req, endLine...)
}

// 编码二进制数组，作为字符串处理
func (c *Cmd) AddBytes(b []byte) {
	c.cmd++
	// '$'
	c.req = append(c.req, '$')
	// length
	n := int64(len(b))
	c.req = strconv.AppendInt(c.req, n, 10)
	// \r\n
	c.req = append(c.req, endLine...)
	if n < 1 {
		return
	}
	// b
	c.req = append(c.req, b...)
	// \r\n
	c.req = append(c.req, endLine...)
}

// 格式化浮点数，作为字符串处理
func (c *Cmd) AddFloat(n float64) {
	c.fmt = c.fmt[:0]
	c.fmt = strconv.AppendFloat(c.fmt, n, 'f', -1, 64)
	c.AddBytes(c.fmt)
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
		c.AddNil()
		return nil
	}
	switch v.(type) {
	case int8:
		c.AddInt(int64(v.(int8)))
	case uint8:
		c.AddInt(int64(v.(uint8)))
	case int16:
		c.AddInt(int64(v.(int16)))
	case uint16:
		c.AddInt(int64(v.(uint16)))
	case int32:
		c.AddInt(int64(v.(int32)))
	case uint32:
		c.AddInt(int64(v.(uint32)))
	case int64:
		c.AddInt(v.(int64))
	case uint64:
		c.AddInt(int64(v.(uint64)))
	case int:
		c.AddInt(int64(v.(int)))
	case uint:
		c.AddInt(int64(v.(uint)))
	case float32:
		c.AddFloat(float64(v.(float32)))
	case float64:
		c.AddFloat(v.(float64))
	case string:
		c.AddString(v.(string))
	case []byte:
		c.AddBytes(v.([]byte))
	case []interface{}:
		err = c.AddArray(v.([]interface{}))
	default:
		err = c.AddJson(v)
	}
	return
}

// 编码数组
func (c *Cmd) AddArray(a []interface{}) (err error) {
	c.cmd++
	// '*'
	c.req = append(c.req, '*')
	// count
	n := int64(len(a))
	c.req = strconv.AppendInt(c.req, n, 10)
	// \r\n
	c.req = append(c.req, endLine...)
	if n < 1 {
		return
	}
	// item
	for i := 0; i < len(a); i++ {
		err = c.addValue(a[i])
		if err != nil {
			break
		}
	}
	return
}

// 格式化json字符串，作为字符串处理
func (c *Cmd) AddJson(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.AddBytes(data)
	return nil
}

// 编码null对象
func (c *Cmd) AddNil() {
	c.cmd++
	// '$'
	c.req = append(c.req, '$')
	// -1
	c.req = strconv.AppendInt(c.req, -1, 10)
	// \r\n
	c.req = append(c.req, endLine...)
}

// 返回格式化的请求缓存
func (c *Cmd) Data() []byte {
	return c.req
}

// io.WriteTo
func (c *Cmd) WriteTo(writer io.Writer) (int64, error) {
	c.fmt = c.fmt[:0]
	// '*'
	c.fmt = append(c.fmt, '*')
	// count
	c.fmt = strconv.AppendInt(c.fmt, int64(c.cmd), 10)
	// \r\n
	c.fmt = append(c.fmt, endLine...)
	// 先写命令单词的个数
	n, err := writer.Write(c.fmt)
	if err != nil {
		return 0, err
	}
	// 在写命令
	m, err := writer.Write(c.req)
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

// 从r，或者this.res中，读取1行...\r\n
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
		if i >= 0 {
			if c.idx > 0 {
				// 接着上一次没有读完的数据
				c.res = append(c.res, c.buf[:n]...)
				c.idx += i
				return c.res[:c.idx], nil
			}
			// 不拷贝到res
			if n > i {
				// 有多余的数据
				c.res = append(c.res, c.buf[i:n]...)
			}
			// 返回，带\r\n
			return c.buf[:i], nil
		}
		// 没有，继续
		c.res = append(c.res, c.buf[:n]...)
		c.idx = len(c.res)
	}
}

// 从r，或者this.res中，读取n个字节
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
				n := c.idx
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
		if c.idx > 0 {
			// 接着上一次没有读完的数据
			c.res = append(c.res, c.buf[:n]...)
			if int64(len(c.res[c.idx:])) >= m {
				// 有m个字节
				n := c.idx
				c.idx += int(m)
				return c.res[n:c.idx], nil
			}
		} else {
			// 上一次没有数据
			if int64(n) == m {
				// 正好
				return c.buf[:n], nil
			} else if int64(n) > m {
				// 多余的数据，保存
				c.res = append(c.res, c.buf[m:n]...)
				return c.buf[:m], nil
			}
		}
		// 不够，继续
	}
}

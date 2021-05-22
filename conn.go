package redis

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
)

var (
	endLine = []byte{'\r', '\n'}
)

type Error string

func (e Error) Error() string {
	return string(e)
}

func maxInt(i1, i2 int) int {
	if i1 > i2 {
		return i1
	}
	return i2
}

// Why not strconv.ParseInt(),becase b is slice not string.
func parseInt(b []byte) (int64, error) {
	var n int64
	if len(b) > 0 {
		i := 0
		if b[i] == '-' {
			i++
		}
		if '0' <= b[i] && b[i] <= '9' {
			n = int64(b[i] - '0')
			i++
		} else {
			return n, fmt.Errorf("parse int invalid <%q>", b[i])
		}
		for ; i < len(b); i++ {
			if '0' <= b[i] && b[i] <= '9' {
				n *= 10
				n += int64(b[i] - '0')
			} else {
				return n, fmt.Errorf("parse int invalid <%q>", b[i])
			}
		}
		if b[0] == '-' {
			n = 0 - n
		}
	}
	return n, nil
}

// net.Conn and buffer
type conn struct {
	net.Conn
	// Whether this conn is ready for use.
	free bool
	// Use for strconv.Format.
	fmt []byte
	// Command line buffer,or response data buffer.
	buff []byte
	// Begin index of a new command line.
	resIdx1 int
	// Last index of response data parsed.
	resIdx2 int
	// Response data length in buff,resLen<=len(buff)
	resLen int
}

// Write value into buffer.
func (c *conn) WriteValue(a interface{}) {
	switch v := a.(type) {
	case int:
		c.WriteInt(int64(v))
	case uint:
		c.WriteInt(int64(v))
	case int8:
		c.WriteInt(int64(v))
	case uint8:
		c.WriteInt(int64(v))
	case int16:
		c.WriteInt(int64(v))
	case uint16:
		c.WriteInt(int64(v))
	case int32:
		c.WriteInt(int64(v))
	case uint32:
		c.WriteInt(int64(v))
	case int64:
		c.WriteInt(v)
	case uint64:
		c.WriteInt(int64(v))
	case float32:
		c.WriteFloat(float64(v))
	case float64:
		c.WriteFloat(v)
	case string:
		c.WriteString(v)
	case []byte:
		c.WriteBytes(v)
	case []interface{}:
		c.WriteArray(v)
	case nil:
		c.WriteNil()
	default:
		c.WriteJson(v)
	}
}

// Write a simple string value into buffer.
// Example:
// "OK" -> "+OK\r\n".
// Usually from server.
func (c *conn) WriteSimpleString(str string) {
	// '+'
	c.buff = append(c.buff, '+')
	// str
	c.buff = append(c.buff, str...)
	// \r\n
	c.buff = append(c.buff, endLine...)
}

// Write a error value into buffer.
// Example:
// "error" -> "-error\r\n".
// Usually from server.
func (c *conn) WriteError(str string) {
	// '-'
	c.buff = append(c.buff, '-')
	// str
	c.buff = append(c.buff, str...)
	// \r\n
	c.buff = append(c.buff, endLine...)
}

// Write a integer value into buffer.
// Example:
// -100 -> ":-100\r\n".
// However,client can't use this protocal(has been tested),must conver integer to string.
func (c *conn) WriteInt(n int64) {
	c.buff = c.buff[:0]
	c.buff = strconv.AppendInt(c.buff, n, 10)
	c.WriteBytes(c.buff)
}

// See WriteInt().
func (c *conn) WriteFloat(n float64) {
	c.buff = c.buff[:0]
	c.buff = strconv.AppendFloat(c.buff, n, 'f', -1, 64)
	c.WriteBytes(c.buff)
}

// Write a bulk string value into buffer.
// Example:
// "hello" -> "$5\r\nhello\r\n"
// "" -> "$0\r\n\r\n".
func (c *conn) WriteString(str string) {
	// '$'
	c.buff = append(c.buff, '$')
	// len(str)
	c.buff = strconv.AppendInt(c.buff, int64(len(str)), 10)
	// \r\n
	c.buff = append(c.buff, endLine...)
	// str
	c.buff = append(c.buff, str...)
	// \r\n
	c.buff = append(c.buff, endLine...)
}

// See WriteString()
func (c *conn) WriteBytes(buf []byte) {
	// '$'
	c.buff = append(c.buff, '$')
	// len(buf)
	c.buff = strconv.AppendInt(c.buff, int64(len(buf)), 10)
	// \r\n
	c.buff = append(c.buff, endLine...)
	// buf
	c.buff = append(c.buff, buf...)
	// \r\n
	c.buff = append(c.buff, endLine...)
}

// Write array value into buffer.
// Example: []{1,"hello",[]{1,2}}
// len([]) -> "*3\r\n"
// [0] -> ":1\r\n"
// [1] -> "$5\r\nhello\r\n"
// len([2]) -> "*2\r\n"
// [2][0] -> ":1\r\n"
// [2][1] -> ":2\r\n"
func (c *conn) WriteArray(v []interface{}) {
	// '*'
	c.buff = append(c.buff, '*')
	// len(v)
	c.buff = strconv.AppendInt(c.buff, int64(len(v)), 10)
	// \r\n
	c.buff = append(c.buff, endLine...)
	// values
	for _, a := range v {
		c.WriteValue(a)
	}
}

// Write value into buffer.
// First,convert struct to json,
// than,call WriteBytes().
func (c *conn) WriteJson(v interface{}) {
	d, _ := json.Marshal(v)
	c.WriteBytes(d)
}

// Write nil value into buffer.
// Example: nil -> "-1\r\n"
func (c *conn) WriteNil() {
	// -1
	c.buff = strconv.AppendInt(c.buff, -1, 10)
	// \r\n
	c.buff = append(c.buff, endLine...)
}

// Write command args count into buffer.
// Example: "set a 1" -> "*3\r\n..."
func (c *conn) WriteCmdCount(n int64) {
	c.buff = c.buff[:0]
	// '*'
	c.buff = append(c.buff, '*')
	// count
	c.buff = strconv.AppendInt(c.buff, n, 10)
	// \r\n
	c.buff = append(c.buff, endLine...)
}

// Read a complete line.
func (c *conn) readLine() ([]byte, error) {
	// Try to read from buffer.
	b, o := c.tryReadLine()
	if o {
		return b, nil
	}
	// Read from net.Conn and append to buffer.
	var err error
	var n int
	for {
		n, err = c.Conn.Read(c.buff[c.resLen:])
		if err != nil {
			return nil, err
		}
		c.resLen += n
		b, o = c.tryReadLine()
		if o {
			return b, nil
		}
	}
}

// Try to read a complete line from buffer,success return data(exclude \r\n) and true.
func (c *conn) tryReadLine() ([]byte, bool) {
	// Search \r\n
	for ; c.resIdx2 < c.resLen; c.resIdx2++ {
		if c.buff[c.resIdx2] == '\n' && c.buff[c.resIdx2-1] == '\r' {
			c.resIdx2++
			i1 := c.resIdx1
			i2 := c.resIdx2 - 2
			c.resIdx1 = c.resIdx2
			if c.resIdx1 == c.resLen {
				c.resIdx1 = 0
				c.resIdx2 = 0
				c.resLen = 0
			}
			return c.buff[i1:i2], true
		}
	}
	// No enough buffer,resize buffer.
	if c.resIdx2 == len(c.buff) {
		newBuf := make([]byte, len(c.buff)*2)
		copy(newBuf, c.buff)
		c.buff = newBuf
	}
	return nil, false
}

// Read and parse result.
func (c *conn) ReadValue() (interface{}, error) {
	line, err := c.readLine()
	if err != nil {
		return nil, err
	}
	// Command type.
	switch line[0] {
	case '+': // It's a simple string.
		return string(line[1:]), nil
	case '-': // It's a error.
		return nil, Error(line[1:])
	case ':': // It's a integer.
		return parseInt(line[1:])
	case '$': // It's a bulk string.
		var length int64
		length, err = parseInt(line[1:])
		if err != nil {
			return nil, err
		}
		// It's null,and it should return nil,not empty string "".
		if length < 0 {
			return nil, nil
		}
		// Read next line.
		line, err = c.readLine()
		if err != nil {
			return nil, err
		}
		return string(line), nil
	case '*': // It's a array.
		var count int64
		count, err = parseInt(line[1:])
		if err != nil {
			return nil, err
		}
		var array []interface{}
		for i := int64(0); i < count; i++ {
			// Read elements
			var a interface{}
			a, err = c.ReadValue()
			if err != nil {
				return nil, err
			}
			array = append(array, a)
		}
		return array, nil
	default:
		return nil, fmt.Errorf("invalid message type <%q> from server", line[0])
	}
}

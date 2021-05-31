package redis

import (
	"encoding/json"
	"fmt"
	"io"
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
	newLineIdx int
	// Last index of response data that has been parsed.
	parsedIdx int
	// Last index of response data in buff.
	resLenIdx int
}

// Write value into buffer.
func (c *conn) WriteValue(a interface{}) {
	switch v := a.(type) {
	case int:
		c.writeInt(int64(v))
	case uint:
		c.writeInt(int64(v))
	case int8:
		c.writeInt(int64(v))
	case uint8:
		c.writeInt(int64(v))
	case int16:
		c.writeInt(int64(v))
	case uint16:
		c.writeInt(int64(v))
	case int32:
		c.writeInt(int64(v))
	case uint32:
		c.writeInt(int64(v))
	case int64:
		c.writeInt(v)
	case uint64:
		c.writeInt(int64(v))
	case float32:
		c.writeFloat(float64(v))
	case float64:
		c.writeFloat(v)
	case string:
		c.writeString(v)
	case []byte:
		c.writeBytes(v)
	case nil:
		c.writeNil()
	default:
		c.writeJson(v)
	}
}

// Write a integer value into buffer.Convert to string first.
// Example: -100 -> "$4\r\n-100\r\n".
func (c *conn) writeInt(n int64) {
	c.fmt = c.fmt[:0]
	c.fmt = strconv.AppendInt(c.fmt, n, 10)
	c.writeBytes(c.fmt)
}

// Write a float value into buffer.Convert to string first.
// Example: 1.23 -> "$4\r\n-100\r\n".
func (c *conn) writeFloat(n float64) {
	c.fmt = c.fmt[:0]
	c.fmt = strconv.AppendFloat(c.fmt, n, 'f', -1, 64)
	c.writeBytes(c.fmt)
}

// Write a bulk string value into buffer.
// Example:
// "hello" -> "$5\r\nhello\r\n"
// "" -> "$0\r\n\r\n".
func (c *conn) writeString(str string) {
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

// See writeString()
func (c *conn) writeBytes(buf []byte) {
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

// Write value into buffer.
// Convert v to json, than call writeBytes().
func (c *conn) writeJson(v interface{}) {
	d, _ := json.Marshal(v)
	c.writeBytes(d)
}

// Write nil value into buffer.
// Example: nil -> "-1\r\n"
func (c *conn) writeNil() {
	// -1
	c.buff = strconv.AppendInt(c.buff, -1, 10)
	// \r\n
	c.buff = append(c.buff, endLine...)
}

// Write integer into buffer.
// Example: "set a 1" -> "*3\r\n..."
func (c *conn) WriteCount(n int64) {
	// '*'
	c.buff = append(c.buff, '*')
	// count
	c.buff = strconv.AppendInt(c.buff, n, 10)
	// \r\n
	c.buff = append(c.buff, endLine...)
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
		// Read length+2(/r/n) characters.
		line, err = c.readN(int(length) + 2)
		if err != nil {
			return nil, err
		}
		return string(line[:len(line)-2]), nil
	case '*': // It's a array.
		var count int64
		count, err = parseInt(line[1:])
		if err != nil {
			return nil, err
		}
		var array []interface{}
		for i := int64(0); i < count; i++ {
			// Read array elements.
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

// Read a complete line.
func (c *conn) readLine() ([]byte, error) {
	for {
		// Try to read from buffer.
		b, o := c.tryReadLine()
		if o {
			return b, nil
		}
		// Read from net.Conn and append to buffer.
		n, err := c.Conn.Read(c.buff[c.resLenIdx:])
		if err != nil {
			return nil, err
		}
		c.resLenIdx += n
	}
}

// Try to read a complete line from buffer,success return data(exclude \r\n) and true.
func (c *conn) tryReadLine() ([]byte, bool) {
	// Search \r\n
	for ; c.parsedIdx < c.resLenIdx; c.parsedIdx++ {
		if c.buff[c.parsedIdx] == '\n' && c.buff[c.parsedIdx-1] == '\r' {
			c.parsedIdx++
			i1, i2 := c.newLineIdx, c.parsedIdx-2
			c.newLineIdx = c.parsedIdx
			// Reset index when all data has been read.
			if c.newLineIdx == c.resLenIdx {
				c.newLineIdx = 0
				c.parsedIdx = 0
				c.resLenIdx = 0
			}
			return c.buff[i1:i2], true
		}
	}
	// No enough buffer,resize buffer.
	if c.resLenIdx == len(c.buff) {
		if c.newLineIdx == 0 {
			// [newLineIdx...resLenIdx] -> [newLineIdx...resLenIdx...]
			b := make([]byte, 2*len(c.buff))
			copy(b, c.buff)
			c.buff = b
		} else {
			// [...newLineIdx...resLenIdx] -> [newLineIdx...resLenIdx...]
			c.resLenIdx = copy(c.buff, c.buff[c.newLineIdx:c.resLenIdx])
			c.parsedIdx -= c.newLineIdx
			c.newLineIdx = 0
		}
	}
	return nil, false
}

// Read n characters
func (c *conn) readN(n int) ([]byte, error) {
	dataLen := c.resLenIdx - c.newLineIdx
	// If has enough data.
	if dataLen < n {
		buffLeft := len(c.buff) - c.resLenIdx
		dataLeft := n - dataLen
		m := dataLeft - buffLeft
		// If need to grow buffer for read.
		if m > 0 {
			if c.newLineIdx == 0 {
				// [newLineIdx...resLenIdx] -> [newLineIdx...resLenIdx...n]
				newBuff := make([]byte, len(c.buff)+m)
				copy(newBuff, c.buff)
				c.buff = newBuff
			} else {
				if c.newLineIdx >= m {
					// c.newLineIndex + buffLeft >= dataLeft
					// [...newLineIdx...resLenIdx] -> [newLineIdx...resLenIdx...]
					c.resLenIdx = copy(c.buff, c.buff[c.newLineIdx:c.resLenIdx])
				} else {
					// [...newLineIdx...resLenIdx] -> [...newLineIdx...resLenIdx...]
					newBuff := make([]byte, n)
					c.resLenIdx = copy(newBuff, c.buff[c.newLineIdx:c.resLenIdx])
				}
				c.parsedIdx -= c.newLineIdx
				c.newLineIdx = 0
			}
		}
		// Read from net.Conn
		var err error
		m, err = io.ReadAtLeast(c.Conn, c.buff[c.resLenIdx:], dataLeft)
		if err != nil {
			return nil, err
		}
		c.resLenIdx += m
	}
	i := c.newLineIdx
	c.newLineIdx += n
	if c.parsedIdx < c.newLineIdx {
		c.parsedIdx = c.newLineIdx
	}
	// Reset index when all data has been read.
	if c.newLineIdx == c.resLenIdx {
		j := c.newLineIdx
		c.newLineIdx = 0
		c.parsedIdx = 0
		c.resLenIdx = 0
		return c.buff[i:j], nil
	}
	return c.buff[i:c.newLineIdx], nil
}

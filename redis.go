package redis

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

type DialFunc func() (net.Conn, error)

var (
	// 使用了一个关闭的连接池错误
	errClosed  = errors.New("use closed redis connection pool")
	errMaxConn = errors.New("max connection")
)

type dialContext struct {
	timeout time.Duration
	retry   int
	dial    DialFunc
}

// 表示一个redis连接池对象
type Redis struct {
	mtx      sync.Mutex // 同步锁
	freeConn []*conn    // 可用的连接
	allConn  []*conn    // 所有的连接
	maxConn  int        // 连接池的容量
	closed   bool       // 是否关闭
	host     string     // redis服务的地址
	rbuf     int        // 连接的读缓存大小
	dialContext         // dial参数
}

// 关闭连接池
// 返回
// 错误
func (this *Redis) Close() error {
	// 改变状态
	this.mtx.Lock()
	if this.closed {
		this.mtx.Unlock()
		return errClosed
	}
	this.closed = true
	this.mtx.Unlock()
	// 关闭所有连接
	for i := 0; i < len(this.allConn); i++ {
		this.allConn[i].Close()
	}
	return nil
}

// 发送request并接收数据到response
// 都是已经格式化的raw数据
// 参数
// request: 请求命令消息缓存
// response: redis服务响应消息缓存
// 返回
// io读字节
// io写字节
// 错误
func (this *Redis) Cmd(request *Request, response *Response) (int, int, error) {
	// 获取连接
	c, e := this.getConn()
	if nil != e {
		return 0, 0, e
	}
	// 交互
	rn, wn, e := c.Cmd(&this.dialContext, request, response)
	if nil != e {
		c.valid = false
	}
	// 缓存连接
	this.putConn(c)
	return rn, wn, e
}

// Cmd的简单版本
func (this *Redis) Set(key, value string, expire int64) error {
	r1, r2 := GetRequest(), GetResponse()

	r1.String("set").String(key).String(value)
	_, _, e := this.Cmd(r1, r2)
	if nil != e {
		PutRequest(r1)
		PutResponse(r2)
		return e
	}

	if expire > 0 {
		r1.Reset().String("expire").Integer(expire)
		_, _, e := this.Cmd(r1, r2)
		if nil != e {
			PutRequest(r1)
			PutResponse(r2)
			return e
		}
	}

	s, t := r2.Read()
	if t == DataTypeError {
		PutRequest(r1)
		PutResponse(r2)
		return errors.New(s)
	}

	PutRequest(r1)
	PutResponse(r2)

	return nil
}

// Cmd的简单版本
func (this *Redis) Get(key string) (string, error) {
	r1, r2 := GetRequest(), GetResponse()

	r1.String("get").String(key)
	_, _, e := this.Cmd(r1, r2)
	if nil != e {
		PutRequest(r1)
		PutResponse(r2)
		return "", e
	}

	s, t := r2.Read()
	if t == DataTypeError {
		PutRequest(r1)
		PutResponse(r2)
		return "", errors.New(s)
	}

	PutRequest(r1)
	PutResponse(r2)

	return s, nil
}

// Cmd的简单版本
func (this *Redis) GetTo(key string, buf io.Writer) (DataType, error) {
	r1, r2 := GetRequest(), GetResponse()

	t := DataTypeNil

	r1.String("get").String(key)
	_, _, e := this.Cmd(r1, r2)
	if nil == e {
		t = r2.ReadTo(buf)
	}

	PutRequest(r1)
	PutResponse(r2)

	return t, e
}

// 默认的连接方式
// 返回
// 底层连接
// 错误
func (this *Redis) defaultDial() (net.Conn, error) {
	return net.DialTimeout("tcp", this.host, this.dialContext.timeout)
}

// 获取一个可用的连接
func (this *Redis) getConn() (*conn, error) {
	this.mtx.Lock()
	// 是否关闭
	if this.closed {
		this.mtx.Unlock()
		return nil, errClosed
	}
	// 是否有可用的连接
	n := len(this.freeConn) - 1
	if n >= 0 {
		c := this.freeConn[n]
		this.freeConn = this.freeConn[:n]
		this.mtx.Unlock()
		return c, nil
	}
	// 所有连接是否最大
	if len(this.allConn) >= this.maxConn {
		this.mtx.Unlock()
		return nil, errMaxConn
	}
	// 新的连接
	c := &conn{rbuf: make([]byte, this.rbuf)}
	// 加入连接池
	this.allConn = append(this.allConn, c)
	this.mtx.Unlock()
	return c, nil
}

// 回收连接
func (this *Redis) putConn(c *conn) {
	this.mtx.Lock()
	this.freeConn = append(this.freeConn, c)
	this.mtx.Unlock()
}

// 新建一个连接池
// 如果指定了dial函数，cfg中创建连接的相关配置无效
// 参数
// cfg: 初始化配置
// dial: 连接函数
// 返回
// Redis对象
func New(cfg *Config, dial DialFunc) (*Redis) {
	db := new(Redis)
	// dial函数
	db.dial = dial
	if nil == db.dial {
		db.dial = db.defaultDial
	}
	if nil == cfg {
		cfg = new(Config)
	}
	// redis服务地址
	db.host = judgeString(cfg.Host, "127.0.0.1:6379")
	// 最大连接数
	db.maxConn = judgeInt(cfg.MaxConn, 1, 64)
	// io超时
	db.dialContext.timeout = time.Duration(judgeInt(cfg.IOTimeout, 1, 3)) * time.Second
	// 失败重连
	db.dialContext.retry = judgeInt(cfg.RetryConn, 1, 1)
	// 每个连接的读缓存
	db.rbuf = judgeInt(cfg.ReadBuffer, 1, 64)
	// 连接池
	return db
}

func judgeInt(n, min, max int) int {
	if n < min {
		n = max
	}
	return n
}

func judgeString(s1, s2 string) string {
	if s1 == "" {
		return s2
	}
	return s1
}

func parseInt(b []byte) (n int) {
	// 这里假设redis的返回一定（事实上）正确，减少判断
	//if c > '9' || c < '0' {
	//
	//}
	if b[0] == '-' {
		n = int(b[1] - '0')
		for i := 2; i < len(b); i++ {
			n = 10*n + int(b[i]-'0')
		}
		n = 0 - n
	} else {
		n = int(b[0] - '0')
		for i := 1; i < len(b); i++ {
			n = 10*n + int(b[i]-'0')
		}
	}
	return
}

// 将n格式化写入b，b保证有21个字节长度
func formatInt(b []byte, n int64) int {
	i := 0
	// 负数
	if n < 0 {
		b[i] = '-'
		n = 0 - n
		i = 1
	}
	// 写入数组
	for n > 0 {
		b[i] = byte('0' + n%10)
		i++
		n /= 10
	}
	// 反转
	c := byte(0)
	i1 := 0
	i2 := i - 1
	for i1 < i2 {
		c = b[i1]
		b[i1] = b[i2]
		b[i2] = c
		i2--
		i1++
	}
	// 换行crlf
	return i
}

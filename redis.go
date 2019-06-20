package redis

import (
	"errors"
	"net"
	"sync"
	"time"
)

type DialFunc func() (net.Conn, error)

var (
	// 使用了一个关闭的连接池错误
	errClosed = errors.New("use closed redis connection pool")
)

// 表示一个redis连接池对象
type Redis struct {
	conn   chan *conn    // 连接池
	dial   DialFunc      // 指定连接函数
	closed bool          // 是否关闭
	mtx    sync.Mutex    // 同步锁
	host   string        // redis服务的地址
	ioTO   time.Duration // io超时
}

// 关闭连接池
// 返回
// 错误
func (this *Redis) Close() error {
	this.mtx.Lock()
	if this.closed {
		this.mtx.Unlock()
		return errClosed
	}
	this.closed = true
	this.mtx.Unlock()

	close(this.conn)
	for c := range this.conn {
		c.Close()
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
	c, o := <-this.conn
	if !o {
		return 0, 0, errClosed
	}
	// 交互
	rn, wn, e := c.Cmd(this.dial, this.ioTO, request, response)
	if nil != e {
		c.valid = false
	}
	// 缓存连接
	this.conn <- c
	return rn, wn, e
}

// 默认的连接方式
// 返回
// 底层连接
// 错误
func (this *Redis) defaultDial() (net.Conn, error) {
	return net.DialTimeout("tcp", this.host, this.ioTO)
}

//func (this *Redis) Set(key string, value ...interface{}) (int, int, error) {
//	// 消息
//	request := GetMessage()
//	response := GetMessage()
//	// 序列化请求
//	rn, wn, e := this.Cmd(request, response)
//	if nil != e {
//		return rn, wn, e
//	}
//	// 判断返回是否ok
//	return rn, wn, e
//}
//
//func (this *Redis) Get(key string, value ...interface{}) (int, int, error) {
//	// 消息
//	request := GetMessage()
//	response := GetMessage()
//	// 序列化请求
//	rn, wn, e := this.Cmd(request, response)
//	if nil != e {
//		return rn, wn, e
//	}
//	// 判断返回是否ok
//	// 反序列化响应到value数组
//	return rn, wn, e
//}

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
	max_conn := judgeInt(cfg.MaxConn, 1, 64)
	// io超时
	db.ioTO = time.Duration(judgeInt(cfg.IOTimeout, 1, 3)) * time.Second
	// 失败重连
	re_try := judgeInt(cfg.RetryConn, 1, 1)
	// 每个连接的读缓存
	read_buff_len := judgeInt(cfg.ReadBuffer, 1, 64)
	// 连接池
	db.conn = make(chan *conn, max_conn)
	for i := 0; i < max_conn; i++ {
		db.conn <- &conn{
			reConn: re_try,
			rbuf:   make([]byte, read_buff_len),
		}
	}
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

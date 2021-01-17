# redis
redis客户端的实现。  
## 使用
```
import "github.com/qq51529210/redis"

// dial，创建连接的函数。
// db，Client选择的库的索引，0，1，2，3。
// max，连接池的最大连接数，默认是1。
// rto，ReadDealLine，0表示永不超时。
// wto，WriteDealLine，0表示永不超时。
client := NewClient(func() (net.Conn, error) {
    return net.Dial("tcp", "192.168.1.30:6379")
}, 1, 10, time.Second, time.Second)
defer client.Close()

// 命令正确的情况下，err就是网络错误
v, err := c.Do("set", "a", 1)
checkError(err)
if s, o := v.(string); !o && s != "OK" {
	panic("bug")
}

// 命令不正确的情况下，err可能是网络错误，也可能是服务器返回的Error消息
v, err = c.Do("get1", "a")
if s, o := v.(redis.Error); !o {
	panic("bug")
}

// 存的时候，把整数转换成字符串，这里返回的是字符串“1”
v, err = c.Do("get", "a")
checkError(err)
if s, o := v.(string); !o && s != “1” {
	panic("bug")
}

// 访问不存在的值，返回的是nil
v, err = c.Do("get", "b")
checkError(err)
if v != nil {
	panic("bug")
}
```

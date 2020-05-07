# redis
redis客户端的实现。  
## 使用
```
// 首先
// 生成一个客户端对象，这是一个连接池
// 这是一个连接池，可以作为全局对象
// 不需要频繁的NewClient/Close()
client := NewClient(func() (net.Conn, error) {
    return net.Dial("tcp", "192.168.1.30:6379")
}, 10, time.Second, time.Second)
defer client.Close()

// 开始一条新的命令，这里是，set a 1
cmd := client.NewCmd("set")
cmd.String("a")
cmd.Int(1)
// 执行
val, err := client.DoCmd(cmd)
if err != nil {
    t.Fatal(err)
}
// 判断是什么类型的数据
// switch value.(type)
// case int64:
// case error:
// case string:
// case []interface:
// 这里仅仅是服务端返回的OK字符串
// 也有可能是error，比如你命令写错了，set1 a 1
switch val.(type) {
case string:
    if val.(string) != "OK" {
        t.FailNow()
    }
default:
    t.FailNow()
}
// 查询命令，get a
cmd = client.NewCmd("get")
cmd.String("a")
val, err = client.DoCmd(cmd)
if err != nil {
    t.Fatal(err)
}
// 这里为什么返回的是字符串
// 因为Cmd.Int(1)，把整数转成了字符串
// redis定义这个':'（整数）协议，只能服务端返回，不能客户端发送
switch val.(type) {
case string:
    if val.(string) != "1" {
        t.FailNow()
    }
default:
    t.FailNow()
}
```
## 测试
```
goos: darwin
goarch: amd64
pkg: github.com/qq51529210/redis
BenchmarkCmd_SimpleString-4   	125414640	         9.33 ns/op	       0 B/op	       0 allocs/op
BenchmarkCmd_Error-4          	173441132	         6.90 ns/op	       0 B/op	       0 allocs/op
BenchmarkCmd_Int-4            	28084707	        40.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkCmd_String-4         	49877894	        22.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkCmd_Bytes-4          	51342812	        22.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkCmd_Float-4          	 7231250	       164 ns/op	       0 B/op	       0 allocs/op
BenchmarkCmd_Json-4           	 3971719	       302 ns/op	      32 B/op	       1 allocs/op
PASS
```
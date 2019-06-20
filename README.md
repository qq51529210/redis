# redis

<p>使用方法</p>
<pre>
func Test_Redis(t *testing.T) {
	r := New(nil, nil)
	defer r.Close()

	// m1复用要Reset
	m1, m2 := GetRequest(), GetResponse()

	// 命令写在一起
	r.Cmd(m1.Write("set", "a", "12345\r\n第三方速度"), m2)
	s, st := m2.Read()
	t.Logf("type:<%v> value:<%v>", st, s)
	// 命令对的话，应该是收到ok
	if s != "OK" {
		t.FailNow()
	}
	if st != DataTypeResponse {
		t.FailNow()
	}

	// 命令一个一个写
	r.Cmd(m1.Reset().Write("get").Write("a"), m2)
	s, st = m2.Read()
	t.Logf("type:<%v> value:<%v>", st, s)
	if s != "12345" {
		t.FailNow()
	}
	if st != DataTypeString {
		t.FailNow()
	}

	// 里边的实现已经转成了字符串
	m1.Reset().Write("set", "b").Integer(321)
	r.Cmd(m1, m2)
	s, st = m2.Read()
	t.Logf("type:<%v> value:<%v>", st, s)
	// 所以获取到的也是字符串
	r.Cmd(m1.Reset().String("get").String("b"), m2)
	s, st = m2.Read()
	t.Logf("type:<%v> value:<%v>", st, s)
	if s != "321" {
		t.FailNow()
	}
	if st != DataTypeString {
		t.FailNow()
	}

	// 写错命令
	m1.Reset().Write("sett", "b").Integer(321)
	r.Cmd(m1, m2)
	s, st = m2.Read()
	t.Logf("type:<%v> value:<%v>", st, s)
	// 收到错误
	if st != DataTypeError {
		t.FailNow()
	}

	// 请求一个不存在的
	r.Cmd(m1.Reset().String("get").String("nil"), m2)
	s, st = m2.Read()
	t.Logf("type:<%v> value:<%v>", st, s)
	// 结果是nil
	if st != DataTypeNil {
		t.FailNow()
	}

	// 判断
	r.Cmd(m1.Reset().Write("exists", "key"), m2)
	s, st = m2.Read()
	t.Logf("type:<%v> value:<%v>", st, s)
	// 结果是整数
	if st != DataTypeInteger {
		t.FailNow()
	}
	// 0和1，字符串
	if s != "0" {
		t.FailNow()
	}

	// 结果是数组
	r.Cmd(m1.Reset().Write("keys", "*"), m2)
	s, st = m2.Read()
	t.Logf("type:<%v> value:<%v>", st, s)
	// 第一个是数组，和它的长度
	if st != DataTypeArray {
		t.FailNow()
	}
	// 接下来是这个数组里边的元素，有可能还是数组
	for {
		s, st = m2.Read()
		t.Logf("type:<%v> value:<%v>", st, s)
		if st == DataTypeNil {
			break
		}
	}
}
</pre>

<p>下面是测试</p>
<pre>
=== RUN   Test_Redis
--- PASS: Test_Redis (0.01s)
    redis_test.go:15: type:<response> value:<OK>
    redis_test.go:27: type:<string> value:<12345>
    redis_test.go:39: type:<response> value:<OK>
    redis_test.go:43: type:<string> value:<321>
    redis_test.go:55: type:<error> value:<ERR unknown command `sett`, with args beginning with: `b`, `321`, >
    redis_test.go:64: type:<nil> value:<>
    redis_test.go:73: type:<integer> value:<0>
    redis_test.go:86: type:<array> value:<4>
    redis_test.go:94: type:<string> value:<a>
    redis_test.go:94: type:<string> value:<test>
    redis_test.go:94: type:<string> value:<c>
    redis_test.go:94: type:<string> value:<b>
    redis_test.go:94: type:<nil> value:<>
goos: darwin
goarch: amd64
pkg: github.com/qq51529210/redis
BenchmarkSet-4              2000            596168 ns/op               0 B/op          0 allocs/op
BenchmarkGet-4              2000            583515 ns/op               0 B/op          0 allocs/op
PASS
ok      github.com/qq51529210/redis     2.532s

</pre>
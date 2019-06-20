package redis

import "testing"

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

func BenchmarkSet(b *testing.B) {
	r := New(nil, nil)
	defer r.Close()
	m1, m2 := GetRequest(), GetResponse()
	r.Cmd(m1.Reset().Write("set", "test", "test"), m2)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.Cmd(m1.Reset().Write("set", "test", "test"), m2)
	}
}

func BenchmarkGet(b *testing.B) {
	r := New(nil, nil)
	defer r.Close()
	m1, m2 := GetRequest(), GetResponse()
	r.Cmd(m1.Reset().String("set").String("test").String("test"), m2)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.Cmd(m1.Reset().String("get").String("test"), m2)
	}
}

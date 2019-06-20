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
	t.Log(s, st)
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
	t.Log(s, st)
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
	t.Log(s, st)
	// 所以获取到的也是字符串
	r.Cmd(m1.Reset().String("get").String("b"), m2)
	s, st = m2.Read()
	t.Log(s, st)
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
	t.Log(s, st)
	// 收到错误
	if st != DataTypeError {
		t.FailNow()
	}

	// 请求一个不存在的
	r.Cmd(m1.Reset().String("get").String("nil"), m2)
	s, st = m2.Read()
	t.Log(s, st)
	// 结果是nil
	if st != DataTypeNil {
		t.FailNow()
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
	r.Cmd(m1.Reset().Write("set", "test", "test"), m2)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.Cmd(m1.Reset().Write("get", "test"), m2)
	}
}

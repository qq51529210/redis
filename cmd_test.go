package redis

import (
	"bytes"
	"encoding/json"
	"strconv"
	"testing"
)

func TestCmd_SimpleString(t *testing.T) {
	conn := bytes.NewBuffer(nil)
	data := "test Cmd.SimpleString"
	cmd := new(Cmd)
	//
	cmd.SimpleString(data)
	conn.Write(cmd.Cmd())
	//
	val, err := cmd.ReadValue(conn)
	if err != nil {
		t.Fatal(err)
	}
	switch val.(type) {
	case string:
		if val.(string) != data {
			t.FailNow()
		}
	default:
		t.FailNow()
	}
}

func TestCmd_Error(t *testing.T) {
	conn := bytes.NewBuffer(nil)
	data := "test Cmd.Error"
	cmd := new(Cmd)
	//
	cmd.Error(data)
	conn.Write(cmd.Cmd())
	//
	val, err := cmd.ReadValue(conn)
	if err != nil {
		t.Fatal(err)
	}
	switch val.(type) {
	case error:
		if val.(error).Error() != data {
			t.FailNow()
		}
	default:
		t.FailNow()
	}
}

func TestCmd_Int(t *testing.T) {
	conn := bytes.NewBuffer(nil)
	cmd := new(Cmd)
	data := []int64{12345, -12345}
	for i := 0; i < len(data); i++ {
		cmd.Reset()
		//
		cmd.Int(data[i])
		conn.Write(cmd.Cmd())
		//
		val, err := cmd.ReadValue(conn)
		if err != nil {
			t.Fatal(err)
		}
		switch val.(type) {
		case string:
			if val.(string) != strconv.FormatInt(data[i], 10) {
				t.FailNow()
			}
		default:
			t.FailNow()
		}
	}
}

func TestCmd_String(t *testing.T) {
	conn := bytes.NewBuffer(nil)
	data := "test Cmd.String"
	cmd := new(Cmd)
	//
	cmd.String(data)
	conn.Write(cmd.Cmd())
	//
	val, err := cmd.ReadValue(conn)
	if err != nil {
		t.Fatal(err)
	}
	switch val.(type) {
	case string:
		if val.(string) != data {
			t.FailNow()
		}
	default:
		t.FailNow()
	}
}

func TestCmd_Bytes(t *testing.T) {
	conn := bytes.NewBuffer(nil)
	data := "test Cmd.Bytes"
	cmd := new(Cmd)
	//
	cmd.Bytes([]byte(data))
	conn.Write(cmd.Cmd())
	//
	val, err := cmd.ReadValue(conn)
	if err != nil {
		t.Fatal(err)
	}
	switch val.(type) {
	case string:
		if val.(string) != data {
			t.FailNow()
		}
	default:
		t.FailNow()
	}
}

func TestCmd_Float(t *testing.T) {
	conn := bytes.NewBuffer(nil)
	data := 12345.6789
	cmd := new(Cmd)
	//
	cmd.Float(data)
	conn.Write(cmd.Cmd())
	//
	val, err := cmd.ReadValue(conn)
	if err != nil {
		t.Fatal(err)
	}
	switch val.(type) {
	case string:
		if val.(string) != "12345.6789" {
			t.FailNow()
		}
	default:
		t.FailNow()
	}
}

func TestCmd_Array(t *testing.T) {
	conn := bytes.NewBuffer(nil)
	data := []interface{}{
		1, "a", 1.23, []byte("b"),
	}
	cmd := new(Cmd)
	//
	err := cmd.Array(data)
	if err != nil {
		t.Fatal(err)
	}
	conn.Write(cmd.Cmd())
	//
	val, err := cmd.ReadValue(conn)
	if err != nil {
		t.Fatal(err)
	}
	switch val.(type) {
	case []interface{}:
		vs := val.([]interface{})
		// 1
		{
			switch vs[0].(type) {
			case string:
				if vs[0].(string) != "1" {
					t.FailNow()
				}
			default:
				t.FailNow()
			}
		}
		// "a"
		{
			switch vs[1].(type) {
			case string:
				if vs[1].(string) != "a" {
					t.FailNow()
				}
			default:
				t.FailNow()
			}
		}
		// 1.23
		{
			switch vs[2].(type) {
			case string:
				if vs[2].(string) != "1.23" {
					t.FailNow()
				}
			default:
				t.FailNow()
			}
		}
		// []byte("b")
		{
			switch vs[3].(type) {
			case string:
				if vs[3].(string) != "b" {
					t.FailNow()
				}
			default:
				t.FailNow()
			}
		}
	default:
		t.FailNow()
	}
}

type testJson struct {
	A int
	B string
}

func TestCmd_Json(t *testing.T) {
	conn := bytes.NewBuffer(nil)
	data := &testJson{
		A: 1,
		B: "test Cmd.Json",
	}
	str, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}

	cmd := new(Cmd)
	//
	cmd.Json(data)
	conn.Write(cmd.Cmd())
	//
	val, err := cmd.ReadValue(conn)
	if err != nil {
		t.Fatal(err)
	}
	switch val.(type) {
	case string:
		if val.(string) != string(str) {
			t.FailNow()
		}
	default:
		t.FailNow()
	}
}

func TestCmd_Value(t *testing.T) {
	conn := bytes.NewBuffer(nil)
	cmd := new(Cmd)
	// 整数
	{
		cmd.Reset()
		cmd.Value(123456)
		conn.Write(cmd.Cmd())
		//
		val, err := cmd.ReadValue(conn)
		if err != nil {
			t.Fatal(err)
		}
		switch val.(type) {
		case string:
			if val.(string) != "123456" {
				t.FailNow()
			}
		default:
			t.FailNow()
		}
	}
	// 字符串
	{
		cmd.Reset()
		cmd.Value("test Cmd.String")
		conn.Write(cmd.Cmd())
		//
		val, err := cmd.ReadValue(conn)
		if err != nil {
			t.Fatal(err)
		}
		switch val.(type) {
		case string:
			if val.(string) != "test Cmd.String" {
				t.FailNow()
			}
		default:
			t.FailNow()
		}
	}
	// 浮点
	{
		cmd.Reset()
		cmd.Value(12345.6789)
		conn.Write(cmd.Cmd())
		//
		val, err := cmd.ReadValue(conn)
		if err != nil {
			t.Fatal(err)
		}
		switch val.(type) {
		case string:
			if val.(string) != "12345.6789" {
				t.FailNow()
			}
		default:
			t.FailNow()
		}
	}
	// 二进制
	{
		cmd.Reset()
		cmd.Value([]byte("test Cmd.Bytes"))
		conn.Write(cmd.Cmd())
		//
		val, err := cmd.ReadValue(conn)
		if err != nil {
			t.Fatal(err)
		}
		switch val.(type) {
		case string:
			if val.(string) != "test Cmd.Bytes" {
				t.FailNow()
			}
		default:
			t.FailNow()
		}
	}
	// Json
	{
		data := &testJson{
			A: 1,
			B: "test Cmd.Json",
		}
		str, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		cmd.Reset()
		cmd.Value(data)
		conn.Write(cmd.Cmd())
		//
		val, err := cmd.ReadValue(conn)
		if err != nil {
			t.Fatal(err)
		}
		switch val.(type) {
		case string:
			if val.(string) != string(str) {
				t.FailNow()
			}
		default:
			t.FailNow()
		}
	}
}

func BenchmarkCmd_SimpleString(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	data := "test Cmd.SimpleString"
	cmd := new(Cmd)
	for i := 0; i < b.N; i++ {
		cmd.Reset()
		cmd.SimpleString(data)
	}
}

func BenchmarkCmd_Error(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	data := "test Cmd.Error"
	cmd := new(Cmd)
	for i := 0; i < b.N; i++ {
		cmd.Reset()
		cmd.Error(data)
	}
}

func BenchmarkCmd_Int(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	data := int64(12345)
	cmd := new(Cmd)
	for i := 0; i < b.N; i++ {
		cmd.Reset()
		cmd.Int(data)
	}
}

func BenchmarkCmd_String(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	data := "test Cmd.String"
	cmd := new(Cmd)
	for i := 0; i < b.N; i++ {
		cmd.Reset()
		cmd.String(data)
	}
}

func BenchmarkCmd_Bytes(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	data := []byte("test Cmd.Bytes")
	cmd := new(Cmd)
	for i := 0; i < b.N; i++ {
		cmd.Reset()
		cmd.Bytes(data)
	}
}

func BenchmarkCmd_Float(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	data := 12345.6789
	cmd := new(Cmd)
	for i := 0; i < b.N; i++ {
		cmd.Reset()
		cmd.Float(data)
	}
}

func BenchmarkCmd_Json(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	data := &testJson{
		A: 1,
		B: "test Cmd.Json",
	}
	cmd := new(Cmd)
	for i := 0; i < b.N; i++ {
		cmd.Reset()
		cmd.Json(data)
	}
}

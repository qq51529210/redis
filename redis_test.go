package redis

import (
	"encoding/json"
	"net"
	"strconv"
	"testing"
	"time"
)

func Test_Client(t *testing.T) {
	// Create a new Client
	client, err := NewClient(func(address string) (net.Conn, error) {
		return net.DialTimeout("tcp", address, time.Second)
	}, "redsi://127.0.0.1:6379?db=1")
	if err != nil {
		t.Fatal(err)
	}
	var (
		value interface{}
		str   string
		ok    bool
	)
	// set a 1
	value, err = client.Cmd("set", "a", 1)
	if err != nil {
		t.Fatal(err)
	}
	str, ok = value.(string)
	if !ok || str != "OK" {
		t.FailNow()
	}
	// get a
	value, err = client.Cmd("get", "a")
	if err != nil {
		t.Fatal(err)
	}
	str, ok = value.(string)
	if !ok || str != "1" {
		t.FailNow()
	}
	// set b 1.1
	value, err = client.Cmd("set", "b", 1.1)
	if err != nil {
		t.Fatal(err)
	}
	str, ok = value.(string)
	if !ok || str != "OK" {
		t.FailNow()
	}
	// get b
	value, err = client.Cmd("get", "b")
	if err != nil {
		t.Fatal(err)
	}
	str, ok = value.(string)
	if !ok || str != "1.1" {
		t.FailNow()
	}
	// set c cfg
	type testStruct struct {
		A int     `json:"a"`
		B float64 `json:"b"`
		C string  `json:"c"`
	}
	ts1 := testStruct{
		A: 100,
		B: 2.23,
		C: "test",
	}
	value, err = client.Cmd("set", "c", ts1)
	if err != nil {
		t.Fatal(err)
	}
	str, ok = value.(string)
	if !ok || str != "OK" {
		t.FailNow()
	}
	// get c
	value, err = client.Cmd("get", "c")
	if err != nil {
		t.Fatal(err)
	}
	str, ok = value.(string)
	if !ok {
		t.FailNow()
	}
	var ts2 testStruct
	err = json.Unmarshal([]byte(str), &ts2)
	if err != nil {
		t.Fatal(err)
	}
	if ts2.A != ts1.A || ts2.B != ts1.B || ts2.C != ts1.C {
		t.FailNow()
	}
	// rpush d
	_, err = client.Cmd("del", "d")
	if err != nil {
		t.Fatal(err)
	}
	array := []interface{}{1, "2", 3.3}
	for _, a := range array {
		value, err = client.Cmd("rpush", "d", a)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok = value.(int64); !ok {
			t.FailNow()
		}
	}
	// lrange d
	value, err = client.Cmd("lrange", "d", 0, len(array))
	if err != nil {
		t.Fatal(err)
	}
	newArray, ok := value.([]interface{})
	if !ok || len(newArray) != len(array) {
		t.FailNow()
	}
	if str, ok = newArray[0].(string); !ok || str != strconv.FormatInt(int64(array[0].(int)), 10) {
		t.FailNow()
	}
	if str, ok = newArray[1].(string); !ok || str != array[1].(string) {
		t.FailNow()
	}
	if str, ok = newArray[2].(string); !ok || str != strconv.FormatFloat(array[2].(float64), 'f', 1, 64) {
		t.FailNow()
	}
	// get e
	value, err = client.Cmd("get", "e")
	if err != nil {
		t.Fatal(err)
	}
	if value != nil {
		t.FailNow()
	}
}

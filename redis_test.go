package redis

import (
	"encoding/json"
	"net"
	"strconv"
	"testing"
	"time"
)

func Test_Client(t *testing.T) {
	cfg := &ClientConfig{
		Host:         "192.168.1.14:6379",
		DB:           1,
		MaxConn:      10,
		ReadTimeout:  0,
		WriteTimeout: 0,
	}
	// Create a new Client
	client := NewClient(func(address string) (net.Conn, error) {
		return net.DialTimeout("tcp", address, time.Second)
	}, cfg)
	var (
		value interface{}
		err   error
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
	value, err = client.Cmd("set", "c", cfg)
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
	var newCfg ClientConfig
	err = json.Unmarshal([]byte(str), &newCfg)
	if err != nil {
		t.Fatal(err)
	}
	if newCfg.Host != cfg.Host ||
		newCfg.DB != cfg.DB ||
		newCfg.ReadTimeout != cfg.ReadTimeout ||
		newCfg.WriteTimeout != cfg.WriteTimeout {
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
}

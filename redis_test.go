package redis

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func Test_Client(t *testing.T) {
	cfg := &ClientConfig{
		Host:         "192.168.1.14:6379",
		DB:           1,
		MaxConn:      10,
		ReadTimeout:  3000,
		WriteTimeout: 3000,
	}
	// Create a new Client
	client := NewClient(func(address string) (net.Conn, error) {
		return net.DialTimeout("tcp", address, time.Second)
	}, cfg)
	// set a 1
	value, err := client.Cmd("set", "a", 1)
	if err != nil {
		t.Fatal(err)
	}
	str, ok := value.(string)
	if !ok || str != "ok" {
		t.FailNow()
	}
	// set b 1.1
	value, err = client.Cmd("set", "b", 1.1)
	if err != nil {
		t.Fatal(err)
	}
	str, ok = value.(string)
	if !ok || str != "ok" {
		t.FailNow()
	}
	// set c cfg
	value, err = client.Cmd("set", "c", cfg)
	if err != nil {
		t.Fatal(err)
	}
	str, ok = value.(string)
	if !ok || str != "ok" {
		t.FailNow()
	}
	// set d []interface{}{1, "1", 1.1}
	array := []interface{}{1, "1", 1.1}
	value, err = client.Cmd("set", "d", array)
	if err != nil {
		t.Fatal(err)
	}
	str, ok = value.(string)
	if !ok || str != "ok" {
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
	// get b
	value, err = client.Cmd("get", "b")
	if err != nil {
		t.Fatal(err)
	}
	str, ok = value.(string)
	if !ok || str != "1.1" {
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
	// get d
	value, err = client.Cmd("get", "b")
	if err != nil {
		t.Fatal(err)
	}
	newArray, ok := value.([]interface{})
	if !ok {
		t.FailNow()
	}
	if len(newArray) != len(array) {
		t.FailNow()
	}
	item1, ok := newArray[0].(int64)
	if !ok || item1 != int64(array[0].(int)) {
		t.FailNow()
	}
	item2, ok := newArray[1].(string)
	if !ok || item2 != array[1].(string) {
		t.FailNow()
	}
	item3, ok := newArray[2].(float64)
	if !ok || item3 != array[2].(float64) {
		t.FailNow()
	}
}

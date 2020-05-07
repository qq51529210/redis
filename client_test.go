package redis

import (
	"net"
	"testing"
	"time"
)

// 生成一个Client对象，
// 这是一个连接池，可以作为全局对象
// 不需要频繁的NewClient/Close()
func TestClient(t *testing.T) {
	client := NewClient(func() (net.Conn, error) {
		return net.Dial("tcp", "192.168.1.30:6379")
	}, 10, time.Second, time.Second)
	defer client.Close()
	//
	cmd := client.NewCmd("set")
	cmd.String("a")
	cmd.Int(1)
	val, err := client.DoCmd(cmd)
	if err != nil {
		t.Fatal(err)
	}
	switch val.(type) {
	case string:
		if val.(string) != "OK" {
			t.FailNow()
		}
	default:
		t.FailNow()
	}
	//
	cmd = client.NewCmd("get")
	cmd.String("a")
	val, err = client.DoCmd(cmd)
	if err != nil {
		t.Fatal(err)
	}
	switch val.(type) {
	case string:
		if val.(string) != "1" {
			t.FailNow()
		}
	default:
		t.FailNow()
	}
	t.Log(val)
}

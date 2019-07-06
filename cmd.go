package redis

import (
	"errors"
)

func (this *Redis) simpleCmd(msg *Message, cmd ...string) error {
	n := len(cmd)
	if n < 1 {
		return nil
	}
	m := GetMessage()
	for i := 0; i < n; i++ {
		m.Request.String(cmd[i])
	}
	_, _, e := this.Cmd(&m.Request, &m.Response)
	if nil != e {
		return e
	}
	m.Builder.Reset()
	t, _ := m.Response.ReadTo(&m.Builder)
	if t == DataTypeError {
		return errors.New(m.Builder.String())
	}
	return nil
}

// Cmd的简单版本
func (this *Redis) Set(key, value string, expire int64) error {
	m := GetMessage()
	m.Request.String("set").String(key).String(value)
	_, _, e := this.Cmd(&m.Request, &m.Response)
	if nil != e {
		PutMessage(m)
		return e
	}
	m.Builder.Reset()
	t, _ := m.Response.ReadTo(&m.Builder)
	if t == DataTypeError {
		PutMessage(m)
		return errors.New(m.Builder.String())
	}

	if expire > 0 {
		m.Request.Reset().String("expire").Integer(expire)
		_, _, e := this.Cmd(&m.Request, &m.Response)
		if nil != e {
			PutMessage(m)
			return e
		}
		m.Builder.Reset()
		t, _ := m.Response.ReadTo(&m.Builder)
		if t == DataTypeError {
			PutMessage(m)
			return errors.New(m.Builder.String())
		}
	}
	PutMessage(m)
	return nil
}

// Cmd的简单版本
func (this *Redis) Get(key string) (string, error) {
	m := GetMessage()
	e := this.simpleCmd(m, "get", key)
	if nil != e {
		PutMessage(m)
		return "", e
	}
	s := m.Builder.String()
	PutMessage(m)
	return s, e
}

// Cmd的简单版本
func (this *Redis) Exists(key string) (bool, error) {
	m := GetMessage()
	m.Request.String("exists").String(key)
	_, _, e := this.Cmd(&m.Request, &m.Response)
	if nil != e {
		PutMessage(m)
		return false, e
	}
	m.Buffer.Reset()
	t, _ := m.Response.ReadTo(&m.Buffer)
	if t == DataTypeError {
		PutMessage(m)
		return false, e
	}
	b := parseInt(m.Buffer.Bytes()) == 1
	PutMessage(m)
	return b, e
}

package redis

import (
	"errors"
	"io"
)

func (this *Redis) stringCmd(m *Message) error {
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

func (this *Redis) boolCmd(m *Message) (bool, error) {
	_, _, e := this.Cmd(&m.Request, &m.Response)
	if nil != e {
		return false, e
	}
	m.Buffer.Reset()
	t, _ := m.Response.ReadTo(&m.Buffer)
	if t == DataTypeError {
		m.Builder.Reset()
		io.Copy(&m.Builder, &m.Buffer)
		return false, errors.New(m.Builder.String())
	}
	return parseInt(m.Buffer.Bytes()) == 1, nil
}

func (this *Redis) Set(key, value string) error {
	m := GetMessage()
	m.Request.String("set").String(key).String(value)
	e := this.stringCmd(m)
	PutMessage(m)
	return e
}

func (this *Redis) Get(key string) (string, error) {
	m := GetMessage()
	m.Request.Write("get", key)
	e := this.stringCmd(m)
	if nil != e {
		PutMessage(m)
		return "", e
	}
	s := m.Builder.String()
	PutMessage(m)
	return s, e
}

func (this *Redis) Exists(key string) (bool, error) {
	m := GetMessage()
	m.Request.Write("exists", key)
	b, e := this.boolCmd(m)
	PutMessage(m)
	return b, e
}

func (this *Redis) Del(key string) (bool, error) {
	m := GetMessage()
	m.Request.Write("del", key)
	b, e := this.boolCmd(m)
	PutMessage(m)
	return b, e
}

func (this *Redis) Expire(key string, expire int64) (bool, error) {
	m := GetMessage()
	m.Request.String("expire").String(key).Integer(expire)
	b, e := this.boolCmd(m)
	PutMessage(m)
	return b, e
}

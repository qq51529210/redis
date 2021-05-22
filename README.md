# redis-client

A redis client package written in Golang.

## Example

[Details code](./redis_test.go)

```go
import "github.com/qq51529210/redis"

// Create a new client
client := redis.NewClient(nil, &ClientConfig{
  Host: "",
  DB: 1,
  MaxConn: 10,
  ReadTimeout: 3000,
  WriteTimeout: 3000,
})

// Command "set a 1",but integer 1 will convert to string "1".
value, err := client.Cmd("set", "a", 1)
checkError(err)

// Float number also convert to string "1.1".
value, err := client.Cmd("set", "b", 1.1)
checkError(err)

// Set a struct,will convert to json string too.
value, err = client.Cmd("set", "c", &struct{})
checkError(err)

// Set array.
value, err = client.Cmd("set", "d", []interface{}{1, "1", 1.1})
checkError(err)

// Command "get a",value is string "1" not integer 1!
value, err = client.Cmd("get", "a")
checkError(err)

// Float number value is string "1.1" not float 1.1 too.
value, err = client.Cmd("get", "b")
checkError(err)

// Command "get c",value is json string.
value, err = client.Cmd("get", "c")
checkError(err)

// Command "get d",value is []interface{}{}
value, err = client.Cmd("get", "d")
checkError(err)

// Command "get b",but key "b" doesn't existed,value is nil
value, err = client.Cmd("get", "b")
checkError(err)

// Invalid command "gett a",err is server error message.
value, err = client.Cmd("gett", "a")
checkError(err)

```


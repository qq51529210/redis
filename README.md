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

// Int convert to string "1".
value, err := client.Cmd("set", "a", 1)
checkError(err)

// Float also convert to string "1.1".
value, err := client.Cmd("set", "b", 1.1)
checkError(err)

// Struct convert to json.
value, err = client.Cmd("set", "c", &struct{})
checkError(err)

// Value is string "1".
value, err = client.Cmd("get", "a")
checkError(err)

// Value is string "1.1".
value, err = client.Cmd("get", "b")
checkError(err)

// Value is json.
value, err = client.Cmd("get", "c")
checkError(err)

```


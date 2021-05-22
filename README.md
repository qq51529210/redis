# redis-client
A redis client package written in Golang.
## Example

```go
import "github.com/qq51529210/redis"

// Create a new client,
client, err := redis.NewClient(...)
checkError(err)

// Write command "set a 1"
value, err := client.Cmd("set", "a", 1)
checkError(err)

// Write command "get a".But,value is string "1" not integer 1!
value, err = client.Cmd("get", "a")
checkError(err)

// Write command "get b",but key "b" doesn't existed,value is nil
value, err = client.Cmd("get", "b")
checkError(err)

// Write a invalid command "gett a",err is server error message.
value, err = client.Cmd("gett", "a")
checkError(err)
```


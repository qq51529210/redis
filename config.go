package redis

type Config struct {
	Host       string `json:"host"`
	DB         string `json:"db"`
	MaxConn    int    `json:"max_conn"`
	IOTimeout  int    `json:"io_timeout"`
	RetryConn  int    `json:"retry_conn"`
	ReadBuffer int    `json:"read_buffer"`
}

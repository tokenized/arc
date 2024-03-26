package arc

import (
	"strings"
	"time"

	"github.com/pkg/errors"
)

type Config struct {
	ConnectTimeout time.Duration `defautl:"10s" json:"connection_timeout"`
	RequestTimeout time.Duration `defautl:"30s" json:"request_timeout"`
}

func (c Config) Copy() Config {
	return Config{
		ConnectTimeout: c.ConnectTimeout,
		RequestTimeout: c.RequestTimeout,
	}
}

func DefaultConfig() Config {
	return Config{
		ConnectTimeout: time.Second * 10,
		RequestTimeout: time.Second * 30,
	}
}

type Factory struct {
	config Config
}

func NewFactory(config Config) *Factory {
	return &Factory{
		config: config,
	}
}

func (f *Factory) NewClient(url, authToken, callBackURL string) (Client, error) {
	if strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return NewHTTPClient(url, authToken, callBackURL, f.config), nil
	}

	return nil, errors.New("Unsupported URL protocol")
}

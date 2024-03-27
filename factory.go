package arc

import (
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tokenized/config"
)

type Config struct {
	ConnectTimeout config.Duration `defautl:"10s" json:"connection_timeout"`
	RequestTimeout config.Duration `defautl:"30s" json:"request_timeout"`
}

func (c Config) Copy() Config {
	return Config{
		ConnectTimeout: c.ConnectTimeout,
		RequestTimeout: c.RequestTimeout,
	}
}

func DefaultConfig() Config {
	return Config{
		ConnectTimeout: config.NewDuration(time.Second * 10),
		RequestTimeout: config.NewDuration(time.Second * 30),
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

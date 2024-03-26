package arc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"

	"github.com/tokenized/arc/pkg/tef"
	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/expanded_tx"
	"github.com/tokenized/pkg/peer_channels"

	"github.com/pkg/errors"
)

const (
	PathPolicy    = "v1/policy"
	PathTxStatus  = "v1/tx/%s"
	PathSubmitTx  = "v1/tx"
	PathSubmitTxs = "v1/txs"

	HeaderKeyCallbackURL       = "X-CallbackUrl"
	HeaderKeyCallbackToken     = "X-CallbackToken"
	HeaderKeyFullStatusUpdates = "X-FullStatusUpdates"
	HeaderKeyWaitForStatus     = "X-WaitForStatus"
)

var (
	httpStatusDescriptions = map[int]string{
		http.StatusBadRequest:          "bad request",
		http.StatusUnauthorized:        "unauthorized",
		http.StatusConflict:            "generic",
		http.StatusUnprocessableEntity: "malformed request",
		460:                            "not extended format",
		461:                            "malformed transaction",
		462:                            "invalid inputs",
		463:                            "malformed transaction",
		464:                            "invalid outputs",
		465:                            "fee too low",
	}
)

type HTTPClient struct {
	url         atomic.Value
	authToken   atomic.Value
	callBackURL atomic.Value

	httpClient *http.Client
}

type HTTPError struct {
	Status      int
	Message     string
	Description string
}

func (err HTTPError) Error() string {
	result := fmt.Sprintf("HTTP Status %d", err.Status)

	if len(err.Description) > 0 {
		result = fmt.Sprintf("%s : %s", result, err.Description)
	}

	if len(err.Message) > 0 {
		result = fmt.Sprintf("%s : %s", result, err.Message)
	}

	return result
}

// Returns true if this error represents an error caused by a tx being invalid.
func IsInvalidTxError(err error) bool {
	httpError, ok := err.(HTTPError)
	if !ok {
		return false
	}

	switch httpError.Status {
	case 461, 462, 463, 464, 465:
		return true
	default:
		return false
	}
}

func NewHTTPClient(url, authToken, callBackURL string, config Config) *HTTPClient {
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: config.ConnectTimeout,
		}).Dial,
		TLSHandshakeTimeout: config.ConnectTimeout,
	}

	result := &HTTPClient{
		httpClient: &http.Client{
			Timeout:   config.RequestTimeout,
			Transport: transport,
		},
	}

	result.url.Store(url)
	result.authToken.Store(authToken)
	result.callBackURL.Store(callBackURL)

	return result
}

func (c HTTPClient) URL() string {
	return c.url.Load().(string)
}

func (c HTTPClient) GetPolicy(ctx context.Context) (*Policy, error) {
	header := make(http.Header)
	if authToken := c.authToken.Load().(string); len(authToken) > 0 {
		header.Add("Authorization", authToken)
	}

	path, err := url.JoinPath(c.url.Load().(string), PathPolicy)
	if err != nil {
		return nil, errors.Wrap(err, "join path")
	}

	policy := &Policy{}
	if err := c.get(path, header, policy); err != nil {
		return nil, errors.Wrap(err, "get")
	}

	return policy, nil
}

func (c HTTPClient) GetTxStatus(ctx context.Context,
	txid bitcoin.Hash32) (*TxStatusResponse, error) {

	header := make(http.Header)
	path, err := url.JoinPath(c.url.Load().(string), fmt.Sprintf(PathTxStatus, txid))
	if err != nil {
		return nil, errors.Wrap(err, "join path")
	}

	response := &TxStatusResponse{}
	if err := c.get(path, header, response); err == nil {
		return response, nil
	}

	return nil, errors.Wrap(err, "get")
}

func (c HTTPClient) SubmitTx(ctx context.Context,
	tx expanded_tx.TransactionWithOutputs) (*TxSubmitResponse, error) {

	buf := &bytes.Buffer{}
	if err := tef.Serialize(buf, tx); err != nil {
		return nil, errors.Wrap(err, "serialize")
	}

	return c.SubmitTxBytes(ctx, buf.Bytes())
}

func (c HTTPClient) SubmitTxBytes(ctx context.Context, txBytes []byte) (*TxSubmitResponse, error) {
	header := make(http.Header)
	if callBackURL := c.callBackURL.Load().(string); len(callBackURL) > 0 {
		peerChannel, err := peer_channels.ParseChannel(callBackURL)
		if err == nil && len(peerChannel.Token) > 0 {
			header.Add(HeaderKeyCallbackURL, peerChannel.MaskedString())
			header.Add(HeaderKeyCallbackToken, peerChannel.Token)
		} else {
			header.Add(HeaderKeyCallbackURL, callBackURL)
		}
		header.Add(HeaderKeyFullStatusUpdates, "true")

		// When using callbacks don't wait for status beyond received to get response.
		header.Add(HeaderKeyWaitForStatus, fmt.Sprintf("%d", int(TxStatusReceived)))
	}

	header.Set("Content-Type", "application/octet-stream")

	path, err := url.JoinPath(c.url.Load().(string), PathSubmitTx)
	if err != nil {
		return nil, errors.Wrap(err, "join path")
	}

	response := &TxSubmitResponse{}
	if err := c.post(path, header, txBytes, response); err == nil {
		return response, nil
	}

	return nil, errors.Wrap(err, "post")
}

func (c HTTPClient) SubmitTxs(ctx context.Context,
	txs []expanded_tx.TransactionWithOutputs) ([]*TxSubmitResponse, error) {

	buf := &bytes.Buffer{}
	for i, tx := range txs {
		if err := tef.Serialize(buf, tx); err != nil {
			return nil, errors.Wrapf(err, "serialize tx %d", i)
		}
	}

	return c.SubmitTxsBytes(ctx, buf.Bytes())
}

func (c HTTPClient) SubmitTxsBytes(ctx context.Context,
	txsBytes []byte) ([]*TxSubmitResponse, error) {

	header := make(http.Header)
	if callBackURL := c.callBackURL.Load().(string); len(callBackURL) > 0 {
		peerChannel, err := peer_channels.ParseChannel(callBackURL)
		if err == nil && len(peerChannel.Token) > 0 {
			header.Add(HeaderKeyCallbackURL, peerChannel.MaskedString())
			header.Add(HeaderKeyCallbackToken, peerChannel.Token)
		} else {
			header.Add(HeaderKeyCallbackURL, callBackURL)
		}
		header.Add(HeaderKeyFullStatusUpdates, "true")

		// When using callbacks don't wait for status beyond received to get response.
		header.Add(HeaderKeyWaitForStatus, fmt.Sprintf("%d", int(TxStatusReceived)))
	}

	path, err := url.JoinPath(c.url.Load().(string), PathSubmitTxs)
	if err != nil {
		return nil, errors.Wrap(err, "join path")
	}

	var response []*TxSubmitResponse
	if err := c.post(path, header, txsBytes, &response); err != nil {
		return nil, errors.Wrap(err, "post")
	}

	return response, nil
}

func (c HTTPClient) get(url string, header http.Header, response interface{}) error {
	httpRequest, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	if authToken := c.authToken.Load().(string); len(authToken) > 0 {
		header.Add("Authorization", authToken)
	}

	for key, values := range header {
		for _, value := range values {
			httpRequest.Header.Add(key, value)
		}
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		if errors.Cause(err) == context.DeadlineExceeded {
			return errors.Wrap(ErrTimeout, errors.Wrap(err, "http get").Error())
		}
		return errors.Wrap(err, "http get")
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode > 299 {
		result := HTTPError{Status: httpResponse.StatusCode}

		if httpResponse.Body != nil {
			b, rerr := ioutil.ReadAll(httpResponse.Body)
			if rerr == nil {
				result.Message = string(b)
			}
		}

		if httpResponse.StatusCode == http.StatusGatewayTimeout {
			return errors.Wrap(ErrTimeout, errors.Wrap(result, "http post").Error())
		}

		if description, exists := httpStatusDescriptions[httpResponse.StatusCode]; exists {
			result.Description = description
		}

		return result
	}

	defer httpResponse.Body.Close()

	if response != nil {
		b, rerr := ioutil.ReadAll(httpResponse.Body)
		if rerr == nil {
			if err := json.Unmarshal(b, response); err != nil {
				return errors.Wrap(err, "unmarshal json")
			}
		}

		// if err := json.NewDecoder(httpResponse.Body).Decode(response); err != nil {
		// 	return errors.Wrap(err, "decode response")
		// }
	}

	return nil
}

func (c HTTPClient) post(url string, header http.Header, request, response interface{}) error {
	var requestReader io.Reader
	if request != nil {
		switch v := request.(type) {
		case string:
			// request is already a json string, not an object to convert to json
			requestReader = bytes.NewReader([]byte(v))
		case []byte:
			// request is already a byte slice, not an object to convert to json
			requestReader = bytes.NewReader(v)
			header.Add("Content-Type", "application/octet-stream")
		default:
			buf := &bytes.Buffer{}
			encoder := json.NewEncoder(buf)
			if err := encoder.Encode(v); err != nil {
				return errors.Wrap(err, "json marshal")
			}
			requestReader = buf
			header.Add("Content-Type", "application/json")
		}
	}

	httpRequest, err := http.NewRequest(http.MethodPost, url, requestReader)
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	if authToken := c.authToken.Load().(string); len(authToken) > 0 {
		header.Add("Authorization", authToken)
	}

	for key, values := range header {
		for _, value := range values {
			httpRequest.Header.Add(key, value)
		}
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		if errors.Cause(err) == context.DeadlineExceeded {
			return errors.Wrap(ErrTimeout, errors.Wrap(err, "http post").Error())
		}
		return errors.Wrap(err, "http post")
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode > 299 {
		result := HTTPError{Status: httpResponse.StatusCode}

		if httpResponse.Body != nil {
			b, rerr := ioutil.ReadAll(httpResponse.Body)
			if rerr == nil {
				result.Message = string(b)
			}
		}

		if httpResponse.StatusCode == http.StatusGatewayTimeout {
			return errors.Wrap(ErrTimeout, errors.Wrap(result, "http post").Error())
		}

		if description, exists := httpStatusDescriptions[httpResponse.StatusCode]; exists {
			result.Description = description
		}

		return result
	}

	defer httpResponse.Body.Close()

	if response != nil {
		b, rerr := ioutil.ReadAll(httpResponse.Body)
		if rerr == nil {
			if err := json.Unmarshal(b, response); err != nil {
				return errors.Wrap(err, "unmarshal json")
			}
		}

		// if err := json.NewDecoder(httpResponse.Body).Decode(response); err != nil {
		// 	return errors.Wrap(err, "decode response")
		// }
	}

	return nil
}

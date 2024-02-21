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
	"path"
	"sync/atomic"

	"github.com/tokenized/arc/pkg/tef"
	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/expanded_tx"

	"github.com/pkg/errors"
)

const (
	PathPolicy    = "v1/policy"
	PathTxStatus  = "v1/tx/%s"
	PathSubmitTx  = "v1/tx"
	PathSubmitTxs = "v1/txs"

	HeaderKeyCallbackURL       = "X-CallbackUrl"
	HeaderKeyFullStatusUpdates = "X-FullStatusUpdates"
	HeaderKeyWaitForStatus     = "X-WaitForStatus"
)

type HTTPClient struct {
	url         atomic.Value
	authToken   atomic.Value
	callBackURL atomic.Value

	httpClient *http.Client
}

type HTTPError struct {
	Status  int
	Message string
}

func (err HTTPError) Error() string {
	if len(err.Message) > 0 {
		return fmt.Sprintf("HTTP Status %d : %s", err.Status, err.Message)
	}

	return fmt.Sprintf("HTTP Status %d", err.Status)
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

	path := path.Join(c.url.Load().(string), PathPolicy)
	policy := &Policy{}
	if err := c.get(path, header, policy); err != nil {
		return nil, errors.Wrap(err, "get")
	}

	return policy, nil
}

func (c HTTPClient) GetTxStatus(ctx context.Context,
	txid bitcoin.Hash32) (*TxStatusResponse, error) {

	header := make(http.Header)
	path := path.Join(c.url.Load().(string), fmt.Sprintf(PathTxStatus, txid))
	response := &TxStatusResponse{}
	err := c.get(path, header, response)
	if err == nil {
		return response, nil
	}

	httpError, ok := err.(HTTPError)
	if !ok {
		return nil, errors.Wrap(err, "get")
	}

	switch httpError.Status {
	case http.StatusUnauthorized: // Security Requirements Failed
	case http.StatusNotFound: // Not Found
	case http.StatusConflict: // Generic Error

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
		header.Add(HeaderKeyCallbackURL, callBackURL)
		header.Add(HeaderKeyFullStatusUpdates, "true")

		// When using callbacks don't wait for status beyond received to get response.
		header.Add(HeaderKeyWaitForStatus, fmt.Sprintf("%d", int(TxStatusReceived)))
	}

	header.Set("Content-Type", "application/octet-stream")

	path := path.Join(c.url.Load().(string), PathSubmitTx)
	response := &TxSubmitResponse{}
	err := c.post(path, header, txBytes, response)
	if err == nil {
		return response, nil
	}

	httpError, ok := err.(HTTPError)
	if !ok {
		return nil, errors.Wrap(err, "post")
	}

	switch httpError.Status {
	case http.StatusBadRequest:
	case http.StatusUnauthorized: // Security Requirements Failed
	case http.StatusConflict: // Generic Error
	case http.StatusUnprocessableEntity: // Malformed request
	case 460: // Not extended format
	case 461: // Malformed transaction
	case 462: // Invalid inputs
	case 463: // Malformed transaction
	case 464: // Invalid outputs
	case 465: // Fee too low
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
		header.Add(HeaderKeyCallbackURL, callBackURL)
		header.Add(HeaderKeyFullStatusUpdates, "true")

		// When using callbacks don't wait for status beyond received to get response.
		header.Add(HeaderKeyWaitForStatus, fmt.Sprintf("%d", int(TxStatusReceived)))
	}

	path := path.Join(c.url.Load().(string), PathSubmitTxs)
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

	httpRequest.Header = header

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

		return result
	}

	defer httpResponse.Body.Close()

	if response != nil {
		b, rerr := ioutil.ReadAll(httpResponse.Body)
		if rerr == nil {
			println("response body", string(b))
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
		default:
			buf := &bytes.Buffer{}
			encoder := json.NewEncoder(buf)
			if err := encoder.Encode(v); err != nil {
				return errors.Wrap(err, "json marshal")
			}
			requestReader = buf
		}
	}

	httpRequest, err := http.NewRequest(http.MethodPost, url, requestReader)
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	if authToken := c.authToken.Load().(string); len(authToken) > 0 {
		header.Add("Authorization", authToken)
	}

	httpRequest.Header = header

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

		return result
	}

	defer httpResponse.Body.Close()

	if response != nil {
		b, rerr := ioutil.ReadAll(httpResponse.Body)
		if rerr == nil {
			println("response body", string(b))
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

package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/tokenized/arc"
	"github.com/tokenized/arc/pkg/tef"
	"github.com/tokenized/channels/peer_channels_listener"
	"github.com/tokenized/config"
	"github.com/tokenized/logger"
	"github.com/tokenized/pkg/expanded_tx"
	"github.com/tokenized/pkg/peer_channels"
	"github.com/tokenized/threads"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type Config struct {
	ListenPeerChannelAccount peer_channels.Account `json:"listen_peer_channel_account"`
	Services                 []Service             `json:"services"`
	Factory                  arc.Config            `json:"factory"`
}

type Service struct {
	URL                 string                `json:"url"`
	AuthToken           string                `json:"auth_token"`
	CallBackPeerChannel peer_channels.Channel `json:"callback_peer_channel"`
}

func main() {
	ctx := logger.ContextWithLogger(context.Background(), true, true, "")

	cfg := &Config{}
	if err := config.LoadConfig(ctx, cfg); err != nil {
		logger.Fatal(ctx, "Failed to load config : %s", err)
	}

	maskedConfig, err := config.MarshalJSONMaskedRaw(cfg)
	if err != nil {
		logger.Fatal(ctx, "Failed to marshal config : %s", err)
	}

	logger.InfoWithFields(ctx, []logger.Field{
		logger.JSON("config", maskedConfig),
	}, "Config")

	if len(os.Args) < 2 {
		logger.Fatal(ctx, "Not enough arguments. Need command (create_send)")
	}

	switch os.Args[1] {
	case "submit":
		if err := Submit(ctx, cfg, os.Args[2:]); err != nil {
			logger.Error(ctx, "Failed to submit tx : %s", err)
		}
	case "listen":
		if err := Listen(ctx, cfg, os.Args[2:]); err != nil {
			logger.Error(ctx, "Failed to listen : %s", err)
		}
	}
}

func Submit(ctx context.Context, cfg *Config, args []string) error {
	if len(args) != 1 {
		logger.Fatal(ctx, "Wrong argument count: submit [tx hex]")
	}

	b, err := hex.DecodeString(args[0])
	if err != nil {
		return errors.Wrap(err, "decode hex")
	}

	r := bytes.NewReader(b)
	var etxs []*expanded_tx.ExpandedTx

	for {
		etx, err := tef.Deserialize(r)
		if err != nil {
			return errors.Wrap(err, "deserialize etx")
		}

		fmt.Printf("Tx : %s\n", etx)
		etxs = append(etxs, etx)

		if r.Len() == 0 {
			break
		}
	}

	factory := arc.NewFactory(cfg.Factory)
	for _, service := range cfg.Services {
		client, err := factory.NewClient(service.URL, service.AuthToken,
			service.CallBackPeerChannel.String())
		if err != nil {
			return errors.Wrapf(err, "new client: %s", service.URL)
		}

		fmt.Printf("Sent %s\n", time.Now())
		response, err := client.SubmitTxsBytes(ctx, b)
		if err != nil {
			return errors.Wrapf(err, "submit tx: %s", service.URL)
		}

		js, _ := json.MarshalIndent(response, "", "  ")
		fmt.Printf("Submit response: %s\n%s\n", service.URL, js)
	}

	return nil
}

func Listen(ctx context.Context, cfg *Config, args []string) error {
	if len(args) != 0 {
		logger.Fatal(ctx, "Wrong argument count: listen")
	}

	fmt.Printf("Listen account : %+v\n", cfg.ListenPeerChannelAccount)

	peerChannelsFactory := peer_channels.NewFactory()
	peerChannelClient, err := peerChannelsFactory.NewClient(cfg.ListenPeerChannelAccount.BaseURL)
	if err != nil {
		return errors.Wrap(err, "peer channel client")
	}

	var wait sync.WaitGroup

	callBackHandler := callBackHandler{cfg}

	listener := peer_channels_listener.NewPeerChannelsListener(peerChannelClient,
		cfg.ListenPeerChannelAccount.Token, 100, callBackHandler.displayCallBack, nil)

	listenerThread, listenerThreadComplete := threads.NewInterruptableThreadComplete("Listener",
		listener.Run, &wait)

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)

	listenerThread.Start(ctx)

	select {
	case <-listenerThreadComplete:
		logger.Error(ctx, "Listener Completed : %s", listenerThread.Error())

	case <-osSignals:
		logger.Info(ctx, "Shutdown requested")
	}

	listenerThread.Stop(ctx)
	wait.Wait()
	return nil
}

type callBackHandler struct {
	cfg *Config
}

func (h callBackHandler) displayCallBack(ctx context.Context, msg peer_channels.Message) error {
	ctx = logger.ContextWithLogTrace(ctx, uuid.New().String())

	fmt.Printf("Received : %s\n", msg.Received)
	fmt.Printf("Sequence : %d\n", msg.Sequence)
	fmt.Printf("ChannelID : %s\n", msg.ChannelID)

	for _, service := range h.cfg.Services {
		if service.CallBackPeerChannel.ChannelID == msg.ChannelID {
			fmt.Printf("Response from url : %s\n", service.URL)
		}
	}

	contentType := msg.BaseContentType()
	if contentType != peer_channels.ContentTypeJSON {
		logger.Warn(ctx, "ARC callback is not JSON : %s", msg.ContentType)
		switch contentType {
		case peer_channels.ContentTypeBinary:
			logger.InfoWithFields(ctx, []logger.Field{
				logger.Hex("payload", msg.Payload),
			}, "Binary payload")
		case peer_channels.ContentTypeText:
			logger.InfoWithFields(ctx, []logger.Field{
				logger.String("payload", string(msg.Payload)),
			}, "Text payload")
		}

		return nil
	}

	rawJS := &bytes.Buffer{}
	if err := json.Indent(rawJS, msg.Payload, "", "  "); err != nil {
		return errors.Wrap(err, "indent json")
	}

	fmt.Printf("Raw Payload : %s\n", string(rawJS.Bytes()))

	callback := &arc.Callback{}
	if err := json.Unmarshal(msg.Payload, callback); err != nil {
		logger.Warn(ctx, "Failed to unmarshal JSON for ARC callback : %s", err)
		return nil
	}

	js, _ := json.MarshalIndent(callback, "", "  ")
	fmt.Printf("Callback : %s\n", js)

	return nil
}

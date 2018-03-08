package forwarder

import (
	"log"
	"github.com/nats-io/go-nats-streaming"
	"time"
	"github.com/nats-io/go-nats"
	"github.com/pkg/errors"
	"net/http"
	"context"
	"io/ioutil"
	"io"
	"encoding/json"
	"bytes"
	"strings"
)
const (
	StrategyFireForget = "fire-forget"
	StrategyACK = "ack"
)

type Forwarder struct {
	StanConfig
	SubscriptionConfig
	stanConn   stan.Conn
	sub        stan.Subscription
	httpClient *http.Client
	ticker     *time.Ticker
	throttle   chan time.Time
}

func (f *Forwarder) makeHttpClient() *http.Client {
	return &http.Client{Timeout: f.Timeout}
}

func (f *Forwarder) Start() error {

	f.httpClient = f.makeHttpClient()
	f.ticker = time.NewTicker(f.RateLimitAsDuration())
	f.throttle = make(chan time.Time, 1)

	if f.stanConn != nil {
		f.stanConn.Close()
	}

	if err := f.connect(); err != nil {
		return err
	}

	go func() {
		for t := range f.ticker.C {
			select {
			case f.throttle <- t:
			default:
			}
		} // does not exit after tick.Stop()
	}()

	if err := f.subscribe(); err != nil {
		return err
	}
	return nil

}

func (f *Forwarder) connect() error {

	natsOpts := nats.GetDefaultOptions()
	natsOpts.AsyncErrorCB = func(c *nats.Conn, s *nats.Subscription, err error) {
		log.Println("Got stan nats async error:", err)
	}

	natsOpts.DisconnectedCB = func(c *nats.Conn) {
		log.Println("Stan Nats disconnected")
	}

	natsOpts.ReconnectedCB = func(c *nats.Conn) {
		log.Println("Stan Nats reconnected")
	}
	natsOpts.Url = f.Url

	natsCon, err := natsOpts.Connect()
	if err != nil {
		return errors.Wrap(err, "nats connect failed")
	}
	if f.stanConn, err = stan.Connect(
		f.ClusterId,
		f.ClientId,
		stan.NatsConn(natsCon),
	); err != nil {
		return errors.Wrap(err, "stan connect failed")
	}

	return nil
}

type RequestPayload struct {
	Subject string `json:"topic"`
	Sequence int `json:"sequence"`
}

func makePayload(msg *stan.Msg) io.Reader {

	bodyBuffer := new(bytes.Buffer)
	json.NewEncoder(bodyBuffer).Encode(msg.MsgProto)
	return bodyBuffer
}

func (f *Forwarder) makeRequest(msg *stan.Msg) {

	var ackSent bool
	data := makePayload(msg)

	req, err := http.NewRequest("POST", f.Endpoint, data)
	if err != nil {
		log.Println("ERROR: ", err)
		return
	}

	ctx, _ := context.WithTimeout(context.Background(), f.Timeout)
	req.WithContext(ctx)
	resp, err := f.httpClient.Do(req)
	if err != nil {
		log.Println("ERROR: ", err)
		return
	}

	if strings.ToLower(f.Strategy) == StrategyFireForget {
		msg.Ack()
		ackSent = true
	}
	// trick to drain body
	defer func() {
		_, _ = io.CopyN(ioutil.Discard, resp.Body, 64)
		_ = resp.Body.Close()
	}()

	ok := false
	for _, goodStatus := range f.HealthyStatus {
		if resp.StatusCode == goodStatus {
			ok = true
			break
		}
	}
	if ! ok {
		return
	}

	if !ackSent {
		msg.Ack()
	}

}

func (f *Forwarder) fwdFunc(msg *stan.Msg) {

	<-f.throttle // rate limit
	log.Printf("got msg on %s", msg.Subject)
	go f.makeRequest(msg)
}

func (f *Forwarder) subscribe() error {

	var err error
	f.sub, err = f.stanConn.QueueSubscribe(
		f.SubscriptionConfig.Subject,
		f.StanConfig.QueueGroupName,
		f.fwdFunc,
		stan.DurableName(f.DurableName),
		stan.SetManualAckMode(),
		stan.AckWait(f.SubscriptionConfig.Timeout+1*time.Second),
	)

	if err != nil {
		return err
	}

	return nil
}

func (f *Forwarder) Stop() {
	if f.stanConn != nil {
		f.stanConn.Close()
	}
}

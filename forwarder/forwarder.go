package forwarder

import (
	"github.com/nats-io/go-nats-streaming"
	"time"
	"net/http"
	"context"
	"io/ioutil"
	"io"
	"encoding/json"
	"bytes"
	"strings"
	log "github.com/sirupsen/logrus"
)
const (
	StrategyFireForget = "fire-forget"
	StrategyACK = "ack"
)

type Forwarder struct {
	log *log.Entry
	StanConfig
	SubscriptionConfig
	StanConn   stan.Conn
	sub        stan.Subscription
	httpClient *http.Client
	ticker     *time.Ticker
	throttle   chan time.Time
}

func (f *Forwarder) makeHttpClient() *http.Client {
	return &http.Client{Timeout: f.Timeout}
}

func (f *Forwarder) Start() error {


	f.log = log.WithFields(log.Fields{
		"subject": f.SubscriptionConfig.Subject,
	})

	rate := f.RateLimitAsDuration()
	f.log.Infof("throttling http requests to 1 / %s", rate)
	f.httpClient = f.makeHttpClient()
	f.ticker = time.NewTicker(rate)
	f.throttle = make(chan time.Time, 1)

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
		log.Error(err)
		return
	}

	for _, fullHeader := range f.SubscriptionConfig.Headers {

		splitH := strings.Split(fullHeader, ":")
		if len(splitH) > 1 {
			f.log.Debugf("setting header `%s`", splitH[0])
			req.Header.Set(splitH[0], splitH[1])
		}
	}

	ctx, _ := context.WithTimeout(context.Background(), f.Timeout)
	req.WithContext(ctx)

	f.log.WithField("url", f.Endpoint)
	f.log.Debug("sending request")
	resp, err := f.httpClient.Do(req)
	if err != nil {
		log.Error(err)
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
	f.log = f.log.WithFields(log.Fields{"seq": msg.Sequence})
	f.log.Debug("got message")
	go f.makeRequest(msg)
}

func (f *Forwarder) subscribe() error {

	var err error


	f.log.Info("subscribing...")

	f.sub, err = f.StanConn.QueueSubscribe(
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

	f.log.Info("subscribed")
	return nil
}

func (f *Forwarder) Stop() {
	if f.StanConn != nil {
		f.StanConn.Close()
	}
}

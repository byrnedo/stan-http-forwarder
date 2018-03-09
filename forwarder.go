package forwarder

import (
	"github.com/nats-io/go-nats-streaming"
	"time"
	"net/http"
	"context"
	"io/ioutil"
	"io"
	"bytes"
	"strings"
	"github.com/sirupsen/logrus"
	"fmt"
)

const (
	StrategyFireForget = "fire-forget"
	StrategyACK        = "ack"
)

type Forwarder struct {
	log        *logrus.Entry
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

	f.log = logrus.New().WithFields(logrus.Fields{
		"subject":     f.SubscriptionConfig.Subject,
		"endpoint":    f.SubscriptionConfig.Endpoint,
		"durableName": f.DurableName,
		"queueGroup":  f.QueueGroup,
	})

	rate := f.RateLimitAsDuration()
	f.log.Infof("throttling http requests to 1/%s", rate)
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

func (f *Forwarder) makeHeaders(msg *stan.Msg) (headers map[string]string) {

	headers = map[string]string{
		"Stan-Seq":       fmt.Sprintf("%d", msg.Sequence),
		"Stan-Subject":   msg.Subject,
		"Stan-Timestamp": fmt.Sprintf("%d", msg.Timestamp),
	}

	for _, fullHeader := range f.SubscriptionConfig.Headers {

		splitH := strings.Split(fullHeader, ":")
		if len(splitH) > 1 {
			headers[splitH[0]] = splitH[1]
		}
	}

	return
}

func (f *Forwarder) makeRequest(msg *stan.Msg) {

	log := f.log.WithField("url", f.Endpoint)

	var ackSent bool
	data := bytes.NewBuffer(msg.MsgProto.Data)

	req, err := http.NewRequest("POST", f.Endpoint, data)
	if err != nil {
		log.Error(err)
		return
	}

	for k, v := range f.makeHeaders(msg) {
		log.Debugf("setting header `%s`", k)
		req.Header.Set(k, v)
	}

	ctx, _ := context.WithTimeout(context.Background(), f.Timeout)
	req.WithContext(ctx)

	log.Debug("sending request")
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
		bodyPreview := make([]byte,256)
		_, _ = resp.Body.Read(bodyPreview)
		log.Warningf("response body: %s", string(bodyPreview))
		return
	}

	if !ackSent {
		msg.Ack()
	}

}

func (f *Forwarder) fwdFunc(msg *stan.Msg) {

	<-f.throttle // rate limit
	f.log = f.log.WithField("seq", msg.Sequence)
	f.log.Debug("got message")
	go f.makeRequest(msg)
}

func (f *Forwarder) subscribe() error {

	var err error

	f.log.Info("subscribing...")

	f.sub, err = f.StanConn.QueueSubscribe(
		f.Subject,
		f.QueueGroup,
		f.fwdFunc,
		stan.DurableName(f.DurableName),
		stan.SetManualAckMode(),
		stan.AckWait(f.Timeout+1*time.Second),
	)

	if err != nil {
		return err
	}

	f.log.Info("subscribed")
	return nil
}

func (f *Forwarder) Stop() {
	if f.sub != nil {
		f.sub.Close()
	}
}

package main

import (
	"flag"
	"github.com/byrnedo/stan-http-forwarder"
	"github.com/nats-io/go-nats"
	"github.com/nats-io/go-nats-streaming"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	config     *forwarder.Config
	forwarders []*forwarder.Forwarder
	log        *logrus.Logger
)

func exitHandler(stanConn stan.Conn) {
	log.Info("caught exit signal, closing connections")
	for _, fwder := range forwarders {
		fwder.Stop()
	}
	stanConn.Close()
	os.Exit(1)
}

func startExitSignalHandler(stanConn stan.Conn) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		//block until signal
		<-c
		exitHandler(stanConn)
	}()
}

func init() {
	log = logrus.New()

	var err error
	confPath := flag.String("conf", "./config.conf", "Path to yaml config file")
	flag.Parse()

	config, err = forwarder.CreateConfig(*confPath)
	if err != nil {
		log.Fatal(err)
	}

}

func connectToStan(sc forwarder.StanConfig) (stan.Conn, error) {

	natsOpts := nats.GetDefaultOptions()
	natsOpts.AsyncErrorCB = func(c *nats.Conn, s *nats.Subscription, err error) {
		log.Error("got stan nats async error:", err)
	}

	natsOpts.DisconnectedCB = func(c *nats.Conn) {
		log.Error("stan Nats disconnected")
	}

	natsOpts.ReconnectedCB = func(c *nats.Conn) {
		log.Info("stan Nats reconnected")
	}
	natsOpts.Url = sc.Url

	natsCon, err := natsOpts.Connect()
	if err != nil {
		return nil, errors.Wrap(err, "nats connect failed")
	}

	var stanConn stan.Conn
	if stanConn, err = stan.Connect(
		sc.ClusterId,
		sc.ClientId,
		stan.NatsConn(natsCon),
	); err != nil {
		return nil, errors.Wrap(err, "stan connect failed")
	}
	return stanConn, nil
}

func main() {

	wg := sync.WaitGroup{}
	wg.Add(1)

	ctxLog := log.WithFields(logrus.Fields{

		"clusterId":   config.Stan.ClusterId,
		"clientId":    config.Stan.ClientId,
	})

	ctxLog.Info("connecting...")
	stanCon, err := connectToStan(config.Stan)
	ctxLog.Info("connected")

	startExitSignalHandler(stanCon)

	logrus.RegisterExitHandler(func() {
		exitHandler(stanCon)
	})

	if err != nil {
		log.Fatal(err)
	}

	ctxLog.Infof("creating %d subscriptions...", len(config.Subscriptions))

	for _, sub := range config.Subscriptions {
		fwder := &forwarder.Forwarder{
			StanConn:           stanCon,
			StanConfig:         config.Stan,
			SubscriptionConfig: sub,
		}

		if err := fwder.Start(); err != nil {
			log.Fatal(err)
		}

		forwarders = append(forwarders, fwder)
	}

	ctxLog.Info("all subscribed")

	wg.Wait()
}

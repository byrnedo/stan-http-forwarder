package main

import (
	"flag"
	"github.com/byrnedo/stan-http-forwarder"
	"log"
	"os"
	"syscall"
	"os/signal"
	"sync"
)

var (
	config *forwarder.Config
	forwarders []*forwarder.Forwarder
)

func init() {

	var err error
	confPath := flag.String("conf", "./config.conf", "Path to yaml config file")
	flag.Parse()


	config, err = forwarder.CreateConfig(*confPath)
	if err != nil {
		log.Fatal(err)
	}

}

func startExitSignalHandler() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		//block until signal
		<-c
		log.Println("caught exit signal, closing connections")
		for _, fwder := range forwarders {
			fwder.Stop()
		}
		os.Exit(1)
	}()
}

func main() {

	startExitSignalHandler()

	wg := sync.WaitGroup{}
	wg.Add(1)

	for _, sub := range config.Subscriptions {
		fwder := &forwarder.Forwarder{
			StanConfig:         config.Stan,
			SubscriptionConfig: sub,
		}

		if err := fwder.Start(); err != nil {
			panic(err)
		}

		forwarders = append(forwarders, fwder)
	}

	wg.Wait()
}

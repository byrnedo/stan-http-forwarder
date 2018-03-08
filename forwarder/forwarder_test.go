package forwarder

import (
	"testing"
	"time"
	"github.com/byrnedo/prefab"

	"github.com/nats-io/go-nats-streaming"
	"github.com/nats-io/go-nats"
)

func initStan(stanUrl string, t *testing.T) stan.Conn {

	natsOpts := nats.GetDefaultOptions()
	natsOpts.Url = stanUrl

	nc,err := natsOpts.Connect()
	if err != nil {
		t.Fatal(err)
	}

	stanConn, err := stan.Connect("test-cluster", "s-h-f-test-producer", stan.NatsConn(nc))
	if err != nil {
		t.Fatal(err)
	}

	return stanConn
}

func TestForwarder_Start(t *testing.T) {

	conId, stanUrl := prefab.StartNatsStreamingContainer()
	defer prefab.Remove(conId)
	if err := prefab.WaitForNatsStreaming(stanUrl, 10*time.Second); err != nil {
		t.Fatal(err)
	}

	stanCon := initStan(stanUrl, t)


	f := &Forwarder{
		StanConfig:         StanConfig{
			Url:            stanUrl,
			ClientId:       "s-h-f-test",
			ClusterId:      "test-cluster",
			DurableName:    "s-h-f",
			QueueGroupName: "s-h-f",
		},
		SubscriptionConfig: SubscriptionConfig{
			Subject:       "foo.bar",
			RateLimit:     "5/1s",
			Strategy:      "ack",
			Endpoint:      "http://localhost:12345",
			HealthyStatus: []int{200},
			Headers:       []string{},
			Timeout:       10 * time.Second,
		},
		StanConn: stanCon,
	}

	if err := f.Start(); err != nil {
		t.Fatal(err)
	}


	for i:=0; i<10; i++ {
		if err := stanCon.Publish("foo.bar", []byte("some data")); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(1*time.Second)


	f.Stop()
}
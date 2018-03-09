package forwarder

import (
	"testing"
	"time"
	"github.com/byrnedo/prefab"

	"github.com/nats-io/go-nats-streaming"
	"github.com/nats-io/go-nats"
	"net/http"
	"io/ioutil"
	"sync"
)

func initStan(stanUrl string, t *testing.T) stan.Conn {

	natsOpts := nats.GetDefaultOptions()
	natsOpts.Url = stanUrl

	nc, err := natsOpts.Connect()
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

	mux := http.NewServeMux()
	wg := sync.WaitGroup{}

	handleFunc := func(w http.ResponseWriter, r *http.Request) {
		b, _ := ioutil.ReadAll(r.Body)
		sub := r.Header.Get("Stan-Subject")
		seq := r.Header.Get("Stan-Seq")

		if sub == "" {
			t.Fatal("missing subject header	")
		}

		if seq == "" {
			t.Fatal("missing sequence header")
		}

		if len(b) == 0 {
			t.Fatal("missing body")
		}
		wg.Done()
	}
	mux.HandleFunc("/", handleFunc)

	go func() {
		if err := http.ListenAndServe("localhost:12345", mux); err != nil {
			t.Fatal(err)
		}
	}()

	f := &Forwarder{
		StanConfig: StanConfig{
			Url:       stanUrl,
			ClientId:  "s-h-f-test",
			ClusterId: "test-cluster",
		},
		SubscriptionConfig: SubscriptionConfig{
			DurableName:   "s-h-f",
			QueueGroup:    "s-h-f",
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

	for i := 0; i < 10; i++ {
		wg.Add(1)
		if err := stanCon.Publish("foo.bar", []byte("some data")); err != nil {
			t.Fatal(err)
		}
	}

	// TODO - add timeout
	wg.Wait()

	f.Stop()
}

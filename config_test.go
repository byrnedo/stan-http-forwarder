package forwarder

import (
	"testing"
)

func TestConfigLoad(t *testing.T) {

	c , err := CreateConfig("./example.conf")
	if err != nil {
		t.Fatal( err)
	}

	if c == nil {
		t.Fatal("config is nil")
	}

	if len(c.Subscriptions) != 2 {
		t.Fatalf("got %d subs", len(c.Subscriptions))
	}

	sub1 := c.Subscriptions[0]
	if sub1.Subject != "foo.bar" {
		t.Fatal("unexpected topic ", sub1.Subject)
	}
	t.Log(c)
}
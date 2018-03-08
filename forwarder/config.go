package forwarder

import (
	"github.com/byrnedo/typesafe-config/parse"
	"time"
	"strings"
	"strconv"
)

type SubscriptionConfig struct {
	Subject       string
	RateLimit     string
	Strategy      string
	Endpoint      string
	HealthyStatus []int
	Headers       []string
	Timeout       time.Duration
}

func (sc *SubscriptionConfig) RateLimitAsDuration() time.Duration {
	splitRateStr := strings.Split(sc.RateLimit, "/")
	if len(splitRateStr) > 1 {

		msgs, _ := strconv.Atoi(splitRateStr[0])
		dur, _ := time.ParseDuration(splitRateStr[1])
		return dur / time.Duration(msgs)
	}

	return time.Second / 10

}

type SubscriptionDefaultConfig struct {
	Strategy string
	RateLimit string
	Timeout   time.Duration
}

type StanConfig struct {
	Url            string
	ClientId       string
	ClusterId      string
	DurableName    string
	QueueGroupName string
}

type Config struct {
	Stan          StanConfig
	Defaults      SubscriptionDefaultConfig
	Subscriptions []SubscriptionConfig
}

func (c *Config) propagateDefaults() {

	if len(c.Stan.ClientId) == 0 {
		c.Stan.ClientId = "stan-http-forwarder-" + randString(5)
	}

	for idx, sub := range c.Subscriptions {
		if sub.Timeout == 0 {
			sub.Timeout = c.Defaults.Timeout
		}
		if sub.RateLimit == "" {
			sub.RateLimit = c.Defaults.RateLimit
		}

		if sub.Strategy == "" {
			sub.Strategy = c.Defaults.Strategy
		}

		c.Subscriptions[idx] = sub
	}

}

func (c *Config) validate() error {

	return nil
}

func CreateConfig(path string) (c *Config, err error) {

	tree, err := parse.ParseFile(path)
	if err != nil {

		return nil, err
	}
	tsConf := tree.GetConfig()

	conf := &Config{}

	parse.Populate(conf, tsConf, "forwarder")
	if err := conf.validate(); err != nil {
		return nil, err
	}
	conf.propagateDefaults()
	return conf, nil
}

# STAN HTTP Proxy

Simple proxy between Nats-Streaming to Http endpoints.

## Config


```yaml
stan:
  url: <nats streaming url>
  user: <nats streaming user>
  pass: <nats streaming pass>

defaults:
  rate: 10/1s # Format: rate/duration. In this case 10 requests per second
  timeout: 5s # Timeout for http requests

subscriptions: # List of subscriptions to stan topics
  - foo.bar: # Stan topic
      rate: 50/1m # Override rate limit
      strategy: ack # Web request strategy, either 'ack' or 'fire-forget'. 'Ack' results in message retry until a healthy status
      endpoint: http://testfoobar.com # Http url
      healthy-status: # List of statuses to consider healthy, useful with 'ack' strategy
        - 200
      headers: # Custom Http headers
        - Header1
        - Header2
        - Header3
  - foo.bar.baz:
      strategy: fire-forget
      endpoint: http://testfoobarbaz.com
      timeout: 10s
      healthy-status:
        - 200
      headers:
        - Header1
        - Header2
        - Header3
```

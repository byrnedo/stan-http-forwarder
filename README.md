# STAN HTTP Proxy

Simple service/package to forward Nats-Streaming messages to HTTP endpoints.

```
             +--------------+
             |              |
             |              |
             |  HTTP API    |
             |              |
             |              |     +-----------+
             |              <-----+           |
             +--------------+     |           |
                                  |  Stan-Http|Forwarder
                                  |           |
                                  |           |
                                  +-----^-----+
                                        |
                                        |
                                        |
+---------------------------------------+----------------------------------->
                                Nats Streaming Subject

```

## Features

### Lightweight

Few dependencies and simple code.

### Rate Limiting

Requests to endpoints are rate limited to avoid accidental spamming.

### Custom Headers

Can be set per endpoint.

### Message Guarantees

When `strategy` is set to `ack`, the forwarder will only ack to nat-streaming if the HTTP response status is in the `healthy-status` list.


## Usage 

`shf -conf ./path.to.config.file`


## Docker

Image available as [byrnedo/stan-http-forwarder](https://hub.docker.com/r/byrnedo/stan-http-forwarder/)

Allows passing of a `CONFIG` env that act as the whole config file.

## Config

This project uses [byrnedo/typesafe-config](https://github.com/byrnedo/typesafe-config), which is a `hocon` format config file reader.


*Example*

```hocon
forwarder {
    stan {
      url = "nats://localhost:4222"
      cluster-id = "<cluster>"
      client-id = "<client id>"
    }

    defaults {
      strategy = ack
      rate-limit = "10/1s"
      timeout= 5s
      durable-name = "<dur name>"
      queue-group ="<group name>"
    }

    subscriptions = [
      {
          subject= foo.bar
          rate-limit= "50/1m"
          strategy= ack
          endpoint= "http://testfoobar.com"
          healthy-status= [ 200 ]
          headers= [
            "Header1",
            "Header2",
            "Header3"
            ]
      },
      {
          subject= foo.bar.baz
          durable-name = "<dur name>"
          queue-group ="<group name>"
          strategy= fire-forget
          endpoint= "http://testfoobarbaz.com"
          timeout= 10s
          healthy-status=[200]
          headers= [
            "Header1",
            "Header2",
            "Header3"
          ]
      }
    ]
}
```

#### Env Substitution

Soft override (only override if env exists):

```hocon
key = "default"
key = ${?SOME_OVERRIDE_ENV}
```

Hard override

```hocon
key = ${SOME_OVERRIDE_ENV}
```

## Message Format

The http request made is always a "POST" request. 
The body of the request is the `MsgProto.Data` payload of the stan message.
Meta data is sent in the following headers:

- Stan-Subject 
- Stan-Sequence

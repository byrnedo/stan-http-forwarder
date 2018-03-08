FROM alpine:3.6

RUN apk update && apk add --no-cache ca-certificates curl

COPY cmd/cmd /opt/service/

COPY docker.conf /opt/conf/

EXPOSE 80

WORKDIR /opt/service

ENTRYPOINT ["./cmd"]

CMD [ "-conf" , "/opt/conf/docker.conf"]

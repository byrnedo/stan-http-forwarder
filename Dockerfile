FROM alpine:3.6

RUN apk update && apk add --no-cache ca-certificates curl

COPY cmd/shf/shf /opt/service/
COPY docker_entrypoint.sh /opt/service/

COPY docker.conf /opt/conf/default.conf

EXPOSE 80

WORKDIR /opt/service

ENTRYPOINT ["./docker_entrypoint.sh"]

CMD [ "-conf" , "/opt/conf/enabled.conf"]

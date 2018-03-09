#!/bin/sh -e

if [ ! -z "$CONFIG" ]; then
    echo "$CONFIG" > /opt/conf/enabled.conf
else
    cp /opt/conf/default.conf /opt/conf/enabled.conf
fi

exec ./shf "$@"

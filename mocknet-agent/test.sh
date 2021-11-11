#!/bin/bash

OUTPUT="$(ip addr | grep "tap0")"
while [ -z "$OUTPUT" ]
do 
    echo ""
done

ip route add 10.1.0.0/16 dev tap0
ip addr add dev tap0 

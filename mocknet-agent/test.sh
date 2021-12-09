#!/bin/sh

start_time=$(date +%s)

OUTPUT="$(ip addr | grep "tap0")"
while [ -z "$OUTPUT" ]
do 
    end_time=$(date +%s)
    cost_time=$[ $end_time-$start_time ]
    echo $cost_time
    if [ $cost_time -gt 5 ]; then
        echo "5 seconds has passed!"
        break 1
    fi
    sleep 1
done


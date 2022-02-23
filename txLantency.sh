#!/bin/bash
for v in  10 50 100 200 600;
do
    for e in 5 ;
    do
        echo "nocust" n=$v e=$e v=$v>>./txLantency.txt
        ./main -n $v -e $e -p n | grep txLatency:>>./txLantency.txt;
    done
done

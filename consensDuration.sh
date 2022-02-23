#!/bin/bash
for v in  400;
do
    for e in 5 ;
    do
        echo "Turbo" n=$v e=$e v=$v>>./acd.txt
        ./main -n $v -e $e  -v=$v| grep acd>>./acd.txt;
    done
done

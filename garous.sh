#!/bin/bash

for n in 10 50 200 600 1200 2000 ;
do
    for e in 120 240;
    do
        echo n=$n e=$e>>./TestTurbo.txt
        ./main -n $n -e $e -p g | grep atps>>./TestTurbo.txt
        echo
    done
done

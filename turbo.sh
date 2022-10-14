#!/bin/bash

for n in 100;
do
    for e in  60;
    do
        echo n=$n e=$e>>./TestTurbo.txt
        ./main -n $n -e $e| grep atps>>./TestTurbo.txt
        echo
    done
done

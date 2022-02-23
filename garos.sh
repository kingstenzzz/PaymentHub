#!/bin/bash

for n in  10 50 100 200 600;
do
    for e in 60;
    do
        echo
        echo n=$n e=$e>>./TestTurbo_target1w_merkle.txt
        ./main -n $n -e $e  | grep atps>>./TestTurbo_target1w_merkle.txt;
    done
done

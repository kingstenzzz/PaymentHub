#!/bin/bash

for n in  10 50 100 200 600;
do
    for e in 5 10 30 600;
    do
        echo
        echo "Trubo" n=$n e=$e>>./TestTurbo.txt
        ./main -n $n -e $e -p t | grep atps>>./TestTurbo.txt;
    done
done

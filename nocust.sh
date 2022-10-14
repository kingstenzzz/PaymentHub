#!/bin/bash

for n in  2000 4000 6000;
do
    for e in 5 10 30 60;
    do
        echo
        echo  n=$n e=$e>>./nocust.txt
        ./main -n $n -e $e  -p n | grep atps>>./nocust.txt;
    done
done

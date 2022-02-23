package main

import (
	"./Nocust"
	"./PayHub"
	"./TURBO"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

var node = []int{20, 50, 100, 150, 300}
var epochtime = []int{5, 10, 30, 60}

func TestNocust(t *testing.T) {
	for _, e := range epochtime {
		for _, n := range node {
			fmt.Printf("node:%v,epoch:%v\n", n, e)
			nocust.Run(n, e)
			output := fmt.Sprintf("node:%v,epoch:%v\n", n, e)
			f, err := os.OpenFile("./TestNocust", os.O_WRONLY|os.O_TRUNC, 0600)
			defer f.Close()
			if err != nil {
				fmt.Println(err.Error())
			} else {
				_, err = f.Write([]byte(output))
			}
		}

	}
}

func TestGarous(t *testing.T) {
	for _, e := range epochtime {
		//for _,n :=range node{
		n := 20
		fmt.Printf("node:%v,epoch:%v\n", n, e)
		tps := PayHub.Run(n, e)
		fmt.Printf("node:%v,epoch:%v,tps%v\n", n, e, tps)

		output := fmt.Sprintf("node:%v,epoch:%v,tps%v\n", n, e, tps)
		f, err := os.OpenFile("./TestGarous.txt", os.O_WRONLY|os.O_APPEND, 0600)
		time.Sleep(time.Duration(100000) * time.Millisecond)

		defer f.Close()
		if err != nil {
			fmt.Println(err.Error())
		} else {
			_, err = f.Write([]byte(output))
			//	}
		}

	}

}

func TestTurbo(t *testing.T) {
	for _, n := range node {
		for _, e := range epochtime {
			//n:=50
			fmt.Printf("node:%v,epoch:%v\n", n, e)
			t.Run("node"+strconv.Itoa(n)+",epoch"+strconv.Itoa(e), func(t *testing.T) {
				tps := TURBO.Run(n, e)
				time.Sleep(time.Duration(30000) * time.Millisecond)
				fmt.Printf("node:%v,epoch:%v,tps%v\n", n, e, tps)
				output := fmt.Sprintf("node:%v,epoch:%v,tps%v\n", n, e, tps)
				f, err := os.OpenFile("./TestTurbo_target1w_merkle.txt", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)

				defer f.Close()
				if err != nil {
					fmt.Println(err.Error())
				} else {
					_, err = f.Write([]byte(output))
				}

			})

		}
	}

}

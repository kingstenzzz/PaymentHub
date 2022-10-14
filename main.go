package main

import (
	_ "crypto/sha256"
	_ "encoding/hex"
	"flag"
	"fmt"
	_ "github.com/ethereum/go-ethereum/crypto"
	"github.com/kingstenzzz/PaymentHub/Nocust"
	"github.com/kingstenzzz/PaymentHub/PayHub"
	"github.com/kingstenzzz/PaymentHub/TURBO"
	_ "net/http/pprof"
)

var numNode int
var epoch int
var vNode int
var protocol string

func init() {
	flag.IntVar(&numNode, "n", 1000, "number of nodes")
	flag.IntVar(&epoch, "e", 10, "epoch in seconds")
	flag.IntVar(&vNode, "vs", 10, "number of v")
	flag.StringVar(&protocol, "p", "n", "protocol")
	flag.Parse()
}

func main() {
	//runtime.GOMAXPROCS(7)

	//go func() {
	//	http.ListenAndServe("localhost:6060", nil)
	//}()
	//ethVerify()
	fmt.Println("numNode: ", numNode)
	fmt.Println("epoch: ", epoch)
	fmt.Println("")

	if protocol == "t" {
		TURBO.Run(numNode, epoch, vNode)
	} else if protocol == "n" {
		Nocust.Run(numNode, epoch)
	} else if protocol == "g" {
		PayHub.Run(numNode, epoch)
	}

}

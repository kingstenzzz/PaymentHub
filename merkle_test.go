package main

import (
	"./TURBO"
	"./Utils"
	"bytes"
	"crypto/sha256"
	_ "crypto/sha256"
	"encoding/gob"
	"fmt"
	"strconv"
	"testing"
	merkletree "github.com/wealdtech/go-merkletree"



)

var txNum = []int{/*100, 1000, 10000,*/ 1}
var nodeNum = []int{/*20, 50, 100, 150,*/ 300}

func Encode(data interface{}) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestMMerkle(t *testing.T) {
	txSet := make([]TURBO.Tx, 100000)
	for i := 0; i < 100; i++ {
		tx := TURBO.Tx{
			Id:       i,
			Epoch:    i,
			Sender:   i,
			Receiver: i + 1,
			Amount:   1,
		}
		txSet = append(txSet, tx)
	}
	txsetByte := make([][]byte, 100000)
	for _, tx := range txSet {
		txbyte, _ := Utils.Encode(tx)
		txsetByte = append(txsetByte, txbyte)
		//log.Printf("Root:%x",txbyte)
	}
	tree, _ := merkletree.New(txsetByte)
	ROOT := tree.Root()
	fmt.Printf("ROOT%X\nlen:%v\n", ROOT, len(txsetByte))

	baz := txsetByte[5]
	proof, err := tree.GenerateProof(baz, 0)
	if err != nil {
		panic(err)
	}
	verified, err := merkletree.VerifyProof(baz, false, proof, [][]byte{ROOT})
	if err != nil {
		panic(err)
	}
	if !verified {
		panic("failed to verify proof for Baz")
	}

	var Balance = make(map[int]int)
	Balance[1] = 2
	Balance[3] = 1
	babyte, _ := Encode(Balance)
	fmt.Printf("%v", babyte)
	//	tree, _ := merkletree.New(babyte)
	//ROOT :=tree.Root()
}

func MerkleRoot(content []byte) []byte {
	return content
}
func ISRTest(numTx, nodeNum int) {

	Balance := make(map[int]int, nodeNum)
	tx := TURBO.Tx{
		Id:       1,
		Sender:   1,
		Receiver: 2,
		Amount:   1,
	}
	txbyte, _ := Utils.Encode(tx)
	SRL := make([][]byte, numTx+1)
	balanceByte := make([][]byte, nodeNum)
	for i, balance := range Balance {
		balance = 1000000
		balancebyte, _ := Utils.Encode(balance)
		balanceByte[i] = balancebyte
	}
	SR1tree, _ := merkletree.New(balanceByte)
	SR1 := SR1tree.Root()
	SRL[0] = SR1
	for i := 0; i < numTx; i++ {
		Balance[1] += tx.Amount
		balancebyte, _ := Utils.Encode(Balance[1])
		balanceByte[1] = balancebyte
		Balance[2] += tx.Amount
		balancebyte, _ = Utils.Encode(Balance[2])
		balanceByte[2] = balancebyte
		SR2tree, _ := merkletree.New(balanceByte)
		SR2 := SR2tree.Root()
		SRL[i] = SR2
	}
	ISRL := make([][]byte, numTx)
	for i := 0; i < numTx-1; i++ {
		IS := sha256.New()
		IS.Write(SRL[i])
		IS.Sum(SRL[i+1])
		ISByte := IS.Sum(txbyte)
		ISRL[i] = ISByte //添加过程根
	}
	ISRtree, _ := merkletree.New(ISRL)
	ISRtree.Root()

}
func benchmarkISR(b *testing.B, txNum int, nodeNum int) {
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		ISRTest(txNum, nodeNum)
	}
}

func BenchmarkISR(b *testing.B) {
	for _, nodenum := range nodeNum {
		for _, txnum := range txNum {
			b.Run("node"+strconv.Itoa(nodenum)+",tx"+strconv.Itoa(txnum), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					b.ReportAllocs()
					//b.ResetTimer()
					ISRTest(txnum, nodenum)
				}
			})

		}

	}
}

/*
func TestMerkle(t *testing.T) {
	numTx := 100000
	c := qt.New(t)
	database, err := badgerdb.New(db.Options{Path: c.TempDir()})
	c.Assert(err, qt.IsNil)

	tree, err := merkle.NewTree(merkle.Config{database, 256, merkle.DefaultThresholdNLeafs,
		merkle.HashFunctionBlake2b})
	c.Assert(err, qt.IsNil)
	defer database.Close() //nolint:errcheck

	Balance := make(map[int]int, 100000)
	tx := TURBO.Tx{
		Id:       1,
		Sender:   1,
		Receiver: 2,
		Amount:   1,
	}
	txbyte, _ := Utils.Encode(tx)
	SRL := make([][]byte, 100000+1)
	balanceByte := make([][]byte, 100000)
	key :=make([][]byte, 100000)
	for i, balance := range Balance {
		balance = 1000000
		balancebyte, _ := Utils.Encode(balance)
		balanceByte[i] = balancebyte
		key[i], _ =Utils.Encode(i)
		invalids, err := tree.AddBatch(key, balanceByte)
		c.Assert(err, qt.IsNil)
		c.Assert(err, qt.IsNil)
		c.Check(len(invalids), qt.Equals, 0)
		root, _ :=tree.Root()
		fmt.Printf("%x",root)
		ISRL := make([][]byte, numTx)
		for i := 0; i < numTx-1; i++ {
			IS := sha256.New()
			IS.Write(SRL[i])
			IS.Sum(SRL[i+1])
			ISByte := IS.Sum(txbyte)
			ISRL[i] = ISByte //添加过程根
		}
		ISRtree, _ := merkletree.New(ISRL)
		ISRtree.Root()



	}




}
*/











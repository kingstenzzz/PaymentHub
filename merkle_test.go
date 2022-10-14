package main

import (
	"bytes"
	"crypto/sha256"
	_ "crypto/sha256"
	"encoding/gob"
	"fmt"
	"github.com/kingstenzzz/PaymentHub/TURBO"
	"github.com/kingstenzzz/PaymentHub/Utils"
	merkle "github.com/vocdoni/arbo"
	merkletree "github.com/wealdtech/go-merkletree"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/badgerdb"
	"log"
	"runtime"
	"strconv"
	"testing"
)

var txNum = []int{ 10000, 50000, 100000, 2000000}
var nodeNum = []int{/*20, 50, 100, 150,*/100}

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
	proof, err := tree.GenerateProof(baz)
	if err != nil {
		panic(err)
	}
	verified, err := merkletree.VerifyProof(baz, proof, ROOT)
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
		maxProces := runtime.NumCPU()
		if maxProces > 1 {
			maxProces -= 1
		}
		runtime.GOMAXPROCS(maxProces)
		ISRTest(txNum, nodeNum)
	}
}

func BenchmarkISR(b *testing.B) {
	for _, nodenum := range nodeNum {
		for _, txnum := range txNum {
			b.Run("node"+strconv.Itoa(nodenum)+",tx"+strconv.Itoa(txnum), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					b.ReportAllocs()
					//b.RunParallel()
					//b.ResetTimer()
					ISRTest(txnum, nodenum)
					//FastMerkle(txnum,nodenum)
				}
			})

		}

	}
}

func FastMerkle(numTx, nodeNum int) {
	database, _ := badgerdb.New(db.Options{Path:"F:\\data\\db1"})
	SR1tree, _ := merkle.NewTree(merkle.Config{database, 256, 100,
		merkle.HashFunctionPoseidon})
	defer database.Close() //nolint:errcheck

	database2, err := badgerdb.New(db.Options{Path:"F:\\data\\db2"})
	if err != nil{
		log.Println(err)
	}
	defer database2.Close() //nolint:errcheck

	SR2tree, _ := merkle.NewTree(merkle.Config{database2, 256, 100,
		merkle.HashFunctionPoseidon})

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
	keyByte :=make([][]byte,nodeNum)
	for i, balance := range Balance {
		balance = 1000000
		balancebyte, _ := Utils.Encode(balance)
		balanceByte[i] = balancebyte
		keyByte[i],_ = Utils.Encode(i)
	}
	SR1tree.AddBatch(keyByte,balanceByte)
	SR1, _ := SR1tree.Root()
	SRL[0] = SR1
	for i := 0; i < numTx; i++ {
		Balance[1] += tx.Amount
		balancebyte, _ := Utils.Encode(Balance[1])
		balanceByte[1] = balancebyte
		Balance[2] += tx.Amount
		balancebyte, _ = Utils.Encode(Balance[2])
		balanceByte[2] = balancebyte
		SR2tree.AddBatch(keyByte,balanceByte)
		SR2, _ := SR2tree.Root()
		SRL[i] = SR2
	}
	database3, _ := badgerdb.New(db.Options{Path:"F:\\data\\db3"})
	ISRtree, _ := merkle.NewTree(merkle.Config{database3, 256, 100,
		merkle.HashFunctionPoseidon})
	defer database3.Close() //nolint:errcheck

	ISRKey := make([][]byte, numTx)
	ISRL := make([][]byte, numTx)
	for i := 0; i < numTx-1; i++ {
		IS := sha256.New()
		IS.Write(SRL[i])
		IS.Sum(SRL[i+1])
		ISByte := IS.Sum(txbyte)
		ISRL[i] = ISByte //添加过程根
	}
	_, err = ISRtree.AddBatch(ISRKey, ISRL)
	if err !=nil{
		log.Printf("err:%v",err)
	}
	ISRROOT,_:=ISRtree.Root()
	fmt.Printf("%x",ISRROOT)
}




package Nocust

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"PaymentHub/Utils"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

var numNode int
var initBalance int

var eonDuration int

var roundChan chan struct{}

var nodesMap map[int]*Node

var availableNodes chan int
var currentTps int
type Stat struct {
	tradingPhaseStartTime   time.Time
	tradingPhaseDuration    time.Duration
	consensusPhaseStartTime time.Time
	consensusPhaseDuration  time.Duration
	tradingPhaseMsgCount    int
	tradingPhaseDataSize    int
	consensusPhaseMsgCount  int
	consensusPhaseDataSize  int

	txCount   int64
	txLatency []int64
	mux       sync.Mutex
}

var stat Stat

type OperatorServer struct {
	priKey  *ecdsa.PrivateKey
	pubAddr string

	eon     int
	balance map[int]int

	txCounter int

	transferChan chan Tx
	receiptChan  chan Tx
}

var OC OperatorServer

func (oc *OperatorServer) cleanUp() {
	for i := len(oc.transferChan); i > 0; i = len(oc.transferChan) {
		<-oc.transferChan
	}
	for i := len(oc.receiptChan); i > 0; i = len(oc.receiptChan) {
		<-oc.receiptChan
	}
	oc.txCounter = 0
	
}

func (oc *OperatorServer) Run() {
	for {
		fmt.Printf("\nenter new eon: %v\n", oc.eon)

		stat.tradingPhaseStartTime = time.Now()

		oc.cleanUp()
		for _, node := range nodesMap {
			node.startTradingChan <- struct{}{}
		}
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(eonDuration)*time.Second)

		nodeTransfer := make(map[int]Tx)
		f := func() {
			for {
				select {
				case <-ctx.Done():
					return
				case transfer := <-oc.transferChan:
					if transfer.eon == oc.eon {
						nodeTransfer[transfer.sender] = transfer
					}
				case receipt := <-oc.receiptChan:
					if receipt.eon != oc.eon {
						break
					}
					if _, ok := nodeTransfer[receipt.sender]; !ok {
						fmt.Printf("oc.Run receipt before transfer %v\n", receipt)
						os.Exit(-1)
					}
					if receipt.receiver != nodeTransfer[receipt.sender].receiver {
						fmt.Printf("operator server fail expecting %v, get %v\n",
							nodeTransfer[receipt.sender], receipt)
						os.Exit(-1)
					}
					transfer := nodeTransfer[receipt.sender]
					oc.balance[receipt.sender] -= receipt.amount
					oc.balance[receipt.receiver] += receipt.amount
					transfer.ocSig = sign(oc.priKey, transfer)
					receipt.ocSig = sign(oc.priKey, receipt)
					go oc.sendConfirm(ctx, transfer.sender, transfer, receipt.receiver, receipt)
				}
			}
		}
		f()

		stat.tradingPhaseDuration = time.Since(stat.tradingPhaseStartTime)
		stat.consensusPhaseStartTime = time.Now()

		roundChan <- struct{}{}

		for nodeId, balance := range oc.balance {
			proof := Proof{eon: oc.eon, nodeId: nodeId, balance: balance, proof: ""}
			proof.ocSig = sign(oc.priKey, proof)
			go nodesMap[nodeId].sendProof(context.TODO(), nodeId, proof)
		}
		Utils.NetworkDelay(0)

		stat.consensusPhaseDuration = time.Since(stat.consensusPhaseStartTime)
		printAndClearStat(oc.eon)

		oc.eon += 1
		currentTps =int(float64(oc.txCounter)/(stat.tradingPhaseDuration.Seconds()+stat.consensusPhaseDuration.Seconds()))
		fmt.Printf("tps: %v\n", int(float64(oc.txCounter)/(stat.tradingPhaseDuration.Seconds()+stat.consensusPhaseDuration.Seconds())))
	}
}

func printAndClearStat(epoch int) {

	fmt.Printf("\nepoch: %v\ntradingDuration: %v, consensusDuration: %v, timeEfficiency: %.2f\ntradingMsgCount: %v, tradingDataSize: %v\nconsensusMsgCount: %v, consensusDataSize: %v\nmsgEfficiency: %f, dataEfficiency: %f\ntxCount: %v, txLatency: %v, stdDev: %v\n",
		epoch,
		stat.tradingPhaseDuration, stat.consensusPhaseDuration,
		float64(stat.tradingPhaseDuration.Milliseconds())/float64(stat.tradingPhaseDuration.Milliseconds()+stat.consensusPhaseDuration.Milliseconds()),
		stat.tradingPhaseMsgCount, stat.tradingPhaseDataSize,
		stat.consensusPhaseMsgCount, stat.consensusPhaseDataSize,
		float64(stat.tradingPhaseMsgCount)/float64(stat.tradingPhaseMsgCount+stat.consensusPhaseMsgCount),
		float64(stat.tradingPhaseDataSize)/float64(stat.tradingPhaseDataSize+stat.consensusPhaseDataSize),
		stat.txCount, Utils.Mean(stat.txLatency), Utils.StdDev(stat.txLatency),
	)
	stat.tradingPhaseMsgCount = 0
	stat.tradingPhaseDataSize = 0
	stat.consensusPhaseMsgCount = 0
	stat.consensusPhaseDataSize = 0
	stat.txCount = 0
	stat.txLatency = []int64{}
}

func (oc *OperatorServer) sendConfirm(ctx context.Context,
	sender int, senderConfirm Tx,
	receiver int, receiverConfirm Tx) {
	Utils.NetworkDelay(0)
	select {
	case <-ctx.Done():
		return
	default:
		stat.tradingPhaseMsgCount++
		stat.tradingPhaseDataSize += Utils.CountBytes(senderConfirm)
		stat.tradingPhaseMsgCount++
		stat.tradingPhaseDataSize += Utils.CountBytes(receiverConfirm)
		oc.txCounter += 1
		nodesMap[sender].confirmChan <- senderConfirm
		nodesMap[receiver].confirmChan <- receiverConfirm
	}
}

type Tx struct {
	eon int

	sender   int
	receiver int
	amount   int

	txSet []Tx

	ocSig       string
	senderSig   string
	receiverSig string

	startTime time.Time
	duration  time.Duration
}

type Node struct {
	id      int
	balance int

	proof Proof
	txSet []Tx

	priKey  *ecdsa.PrivateKey
	pubAddr string

	transferNotifyChan chan Tx
	confirmChan        chan Tx
	proofChan          chan Proof
	taskChan           chan int
	startTradingChan   chan struct{}
}

type Proof struct {
	eon     int
	nodeId  int
	balance int
	ocSig   string
	proof   string
}

func (node *Node) Run() {
	for {
		<-node.startTradingChan
		node.cleanUp()

		ctx, _ := context.WithTimeout(context.Background(), time.Duration(eonDuration)*time.Second)

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case receiver := <-node.taskChan:
					node.senderTx(ctx, receiver)
				}
			}
		}()

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case transferNotify := <-node.transferNotifyChan:
					node.receiverTx(ctx, transferNotify)
				}
			}
		}()

		node.proof = <-node.proofChan
	}
}

func (node *Node) senderTx(ctx context.Context, receiver int) {
	transfer := Tx{
		eon:       node.proof.eon + 1,
		sender:    node.id,
		receiver:  receiver,
		amount:    1,
		txSet:     node.txSet,
		startTime: time.Now(),
	}
	transfer.senderSig = sign(node.priKey, transfer)
	go node.sendTransferToOC(ctx, transfer)
	go node.sendTransferNotify(ctx, receiver, transfer)
	select {
	case <-ctx.Done():
		return
	case txConfirm := <-node.confirmChan:
		txConfirm.duration = time.Since(txConfirm.startTime)
		atomic.AddInt64(&stat.txCount, 1)
		stat.mux.Lock()
		stat.txLatency = append(stat.txLatency, txConfirm.duration.Milliseconds())
		stat.mux.Unlock()
		node.txSet = append(node.txSet, txConfirm)
		node.balance -= txConfirm.amount
		availableNodes <- node.id
	}
}

func (node *Node) receiverTx(ctx context.Context, transferNotify Tx) {
	receipt := Tx{
		eon:       transferNotify.eon,
		sender:    transferNotify.sender,
		receiver:  transferNotify.receiver,
		amount:    transferNotify.amount,
		senderSig: transferNotify.senderSig,
		txSet:     node.txSet,
	}
	receipt.receiverSig = sign(node.priKey, receipt)
	go node.sendReceiptToOC(ctx, receipt)
	select {
	case <-ctx.Done():
		return
	case txConfirm := <-node.confirmChan:
		node.txSet = append(node.txSet, txConfirm)
		node.balance += txConfirm.amount
		availableNodes <- node.id
	}
}

func (node *Node) cleanUp() {
	node.txSet = nil
	for i := len(node.taskChan); i > 0; i = len(node.taskChan) {
		<-node.taskChan
	}
	for i := len(node.transferNotifyChan); i > 0; i = len(node.transferNotifyChan) {
		<-node.transferNotifyChan
	}
	for i := len(node.confirmChan); i > 0; i = len(node.confirmChan) {
		<-node.confirmChan
	}
	availableNodes <- node.id
}

func (node *Node) sendProof(ctx context.Context, to int, proof Proof) {
	bytes := Utils.CountBytes(proof)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		stat.consensusPhaseMsgCount++
		stat.consensusPhaseDataSize += bytes
		nodesMap[to].proofChan <- proof
	}
}

func (node *Node) sendTransferNotify(ctx context.Context, to int, transferNotify Tx) {
	bytes := Utils.CountBytes(transferNotify)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		stat.tradingPhaseMsgCount++
		stat.tradingPhaseDataSize += bytes
		nodesMap[to].transferNotifyChan <- transferNotify
	}
}

func (node *Node) sendTransferToOC(ctx context.Context, transfer Tx) {
	bytes := Utils.CountBytes(transfer)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		stat.tradingPhaseMsgCount++
		stat.tradingPhaseDataSize += bytes
		OC.transferChan <- transfer
	}
}

func (node *Node) sendReceiptToOC(ctx context.Context, receipt Tx) {
	bytes := Utils.CountBytes(receipt)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		stat.tradingPhaseMsgCount++
		stat.tradingPhaseDataSize += bytes
		OC.receiptChan <- receipt
	}
}

func sign(key *ecdsa.PrivateKey, msg interface{}) string {
	msgMarshal, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("sign marshal err=%v\n", err)
		os.Exit(-1)
	}
	msgHash := sha256.Sum256(msgMarshal)
	sig, err := crypto.Sign(msgHash[:], key)
	if err != nil {
		fmt.Printf("sign err=%v\n", err)
		os.Exit(-1)
	}
	return hex.EncodeToString(sig)
}

func initOperatorServer(balanceMap map[int]int) {
	key, err := crypto.GenerateKey()
	if err != nil {
		fmt.Println("initOperatorServer error: ", err.Error())
		os.Exit(1)
	}

	OC = OperatorServer{
		priKey:       key,
		pubAddr:      crypto.PubkeyToAddress(key.PublicKey).Hex(),
		eon:          1,
		balance:      balanceMap,
		transferChan: make(chan Tx, numNode),
		receiptChan:  make(chan Tx, numNode),
		txCounter:    0,
	}
}

func createNode(id int, balance int) *Node {
	key, err := crypto.GenerateKey()
	if err != nil {
		fmt.Println("createNode error: ", err.Error())
		os.Exit(1)
	}

	return &Node{
		id:                 id,
		balance:            balance,
		priKey:             key,
		pubAddr:            crypto.PubkeyToAddress(key.PublicKey).Hex(),
		transferNotifyChan: make(chan Tx, numNode),
		confirmChan:        make(chan Tx, numNode),
		proofChan:          make(chan Proof, 1),
		taskChan:           make(chan int, 1),
		startTradingChan:   make(chan struct{}, 1),
	}
}

func Run(n, e int) {
	fmt.Println("NOCUST...")

	numNode = n
	initBalance = 100000
	nodesMap = make(map[int]*Node)
	roundChan = make(chan struct{}, 1)
	eonDuration = e
	var atps int
	availableNodes = make(chan int, numNode)

	balanceMap := make(map[int]int)
	for i := 0; i < numNode; i++ {
		nodesMap[i] = createNode(i, initBalance)
		balanceMap[i] = initBalance
	}

	initOperatorServer(balanceMap)
	go OC.Run()

	for _, node := range nodesMap {
		go node.Run()
	}

	time.Sleep(time.Second)

	round := 0
	for {
		select {
		case <-roundChan:
			round++
			atps += currentTps
			if round >= 10 {
				fmt.Printf("atps %v",atps/10)
				return
			}
		default:
			sender := <-availableNodes
			receiver := <-availableNodes
			nodesMap[sender].taskChan <- receiver
		}
	}

}

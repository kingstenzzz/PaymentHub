package PayHub

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/kingstenzzz/PaymentHub/Utils"
	"math/rand"
	"os"
	"reflect"
	"time"
)

var numNode int
var initBalance int

var nodesMap map[int]*Node

var finishedTask int
var makeTaskChan chan struct{}

var targetTPS int

var tradingPhaseSecond int

const confNum = 10

type Stat struct {
	tradingPhaseStartTime   time.Time
	tradingPhaseDuration    time.Duration
	consensusPhaseStartTime time.Time
	consensusPhaseDuration  time.Duration
	tradingPhaseMsgCount    int
	tradingPhaseDataSize    int
	consensusPhaseMsgCount  int
	consensusPhaseDataSize  int

	txCount    int64
	txLatency  []int64
	txDuration int64
}

var stat Stat

type Tx struct {
	Id          int
	Epoch       int
	Sender      int
	Receiver    int
	Amount      int
	LeaderSig   string
	SenderSig   string
	ReceiverSig string

	startTime time.Time
	duration  time.Duration
}

type Node struct {
	id      int
	balance int

	state  State
	leader Leader

	priKey  *ecdsa.PrivateKey
	pubAddr string

	txIdRequestChan chan TxIdRequest
	txIdReplyChan   chan TxIdRequest
	txRequestChan   chan Tx
	txReplyChan     chan Tx
	txLeaderChan    chan Tx

	newStateChan        chan State
	newStateReplyChan   chan StateSig
	newStateConfirmChan chan State

	tradingPhaseChan chan struct{}

	taskChan chan struct{}
}

type Leader struct {
	txSet         []Tx
	enrollmentSet map[int]int
	withdrawalSet []int
	txIdGenerator map[int]int
}

func (node *Node) cleanUp() {
	for i := len(node.txIdRequestChan); i > 0; i = len(node.txIdRequestChan) {
		<-node.txIdRequestChan
	}
	for i := len(node.txIdReplyChan); i > 0; i = len(node.txIdReplyChan) {
		<-node.txIdReplyChan
	}
	for i := len(node.txRequestChan); i > 0; i = len(node.txRequestChan) {
		<-node.txRequestChan
	}
	for i := len(node.txReplyChan); i > 0; i = len(node.txReplyChan) {
		<-node.txReplyChan
	}
	for i := len(node.txLeaderChan); i > 0; i = len(node.txLeaderChan) {
		<-node.txLeaderChan
	}
}

func (node *Node) Run(initState State) {
	node.state = initState
	leaderId := initState.LeaderId
	epoch := initState.Epoch
	for {
		node.cleanUp()
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(tradingPhaseSecond)*time.Second)

		// leader election phase
		node.initLeader(ctx, leaderId, epoch)

		<-node.tradingPhaseChan

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-node.taskChan:
					go func() {
						txStartTime := time.Now()
						txId, leaderSig, err := node.getTxId(ctx)
						if err != nil {
							return
						}
						node.senderTx(ctx, txId, leaderSig, txStartTime)
					}()
				}
			}
		}()

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case txRequest := <-node.txRequestChan:
					go node.receiverTx(ctx, txRequest)
				}
			}
		}()

		// consensus phase
		if node.id <= confNum {

			newState := <-node.newStateChan
			//node.verify(newState.LeaderAddr, []byte(newState.Sig[leaderId]),newState.Balance)

			stateSig := StateSig{
				Epoch:  newState.Epoch,
				NodeId: node.id,
				//Sig:    node.signNewState(newState),
				Sig: node.sign(newState),
			}
			node.sendNewStateReply(context.TODO(), node.state.LeaderId, stateSig)
		}

		newStateConfirm := <-node.newStateConfirmChan

		node.state = newStateConfirm
		node.balance = newStateConfirm.Balance[node.id] //修改状态
		leaderId = newStateConfirm.LeaderId
		epoch = newStateConfirm.Epoch
	}
}

func (node *Node) verify(pubkey string, sig []byte, msg interface{}) {
	fmt.Println("VerifyState")

	msgMarshal, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("node %v sign err=%v\n", node.id, err)
		os.Exit(-1)
	}
	msgHash := sha256.Sum256(msgMarshal)
	pkByte := []byte(pubkey)
	ok := crypto.VerifySignature(pkByte, msgHash[:], sig[:len(sig)-1])
	if ok == true {
		fmt.Println("VerifySignature ok")
	} else {
		fmt.Println("VerifySignature error")

	}

}

func (node *Node) sign(msg interface{}) string {
	msgMarshal, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("node %v sign err=%v\n", node.id, err)
		os.Exit(-1)
	}
	msgHash := sha256.Sum256(msgMarshal)
	sig, err := crypto.Sign(msgHash[:], node.priKey)
	if err != nil {
		fmt.Printf("node %v sign err=%v\n", node.id, err)
		os.Exit(-1)
	}
	return hex.EncodeToString(sig)
}

func (node *Node) getRandomNode() int {
	keys := reflect.ValueOf(nodesMap).MapKeys()
	for {
		nodeId := keys[rand.Intn(len(nodesMap))].Interface().(int)
		if nodeId != node.id {
			return nodeId
		}
	}
	return 0
}

func (node *Node) getTxId(ctx context.Context) (int, string, error) {
	txIdRequest := TxIdRequest{
		nodeId:  node.id,
		num:     1,
		idList:  make([]int, 1),
		sigList: make([]string, 1),
	}
	go node.sendTxIdRequest(ctx, node.state.LeaderId, txIdRequest)
	select {
	case <-ctx.Done():
		return 0, "", errors.New("")
	case txIdReply := <-node.txIdReplyChan:
		return txIdReply.idList[0], txIdReply.sigList[0], nil
	}
}

func (node *Node) senderTx(ctx context.Context, txid int, leaderSig string, txStartTime time.Time) {
	tx := Tx{
		Id:        txid,
		Epoch:     node.state.Epoch,
		Sender:    node.id,
		Receiver:  node.getRandomNode(),
		Amount:    1,
		LeaderSig: leaderSig,
		startTime: txStartTime,
	}
	tx.SenderSig = node.sign(tx)

	go node.sendTxRequest(ctx, tx.Receiver, tx)
	select {
	case <-ctx.Done():
		return
	case txReply := <-node.txReplyChan:
		node.sendTxLeader(ctx, node.state.LeaderId, txReply)
	}
}

func (node *Node) receiverTx(ctx context.Context, tx Tx) {
	tx.ReceiverSig = node.sign(tx)
	node.sendTxReply(ctx, tx.Sender, tx)
}

func (node *Node) initLeader(ctx context.Context, leaderId int, epochId int) {
	if node.id != leaderId {
		return
	}
	node.leader.txSet = make([]Tx, 0)
	node.leader.enrollmentSet = make(map[int]int)
	node.leader.withdrawalSet = make([]int, 0)
	node.leader.txIdGenerator = make(map[int]int)
	for nodeId := range nodesMap {
		node.leader.txIdGenerator[nodeId] = 0
	}
	go node.leaderCron(ctx)
	makeTaskChan <- struct{}{}
}

type State struct {
	Epoch      int
	LeaderId   int
	LeaderAddr string
	Balance    map[int]int
	Sig        map[int]string
}

type StateSig struct {
	Epoch  int
	NodeId int
	Sig    string
}

func printState(state State) {
	fmt.Printf("Epoch: %v, LeaderId: %v, Balance: %v, Sig: %v\n",
		state.Epoch, state.LeaderId, state.Balance, state.Sig)
}

func calculateNextState(state State, txSet []Tx, enrollmentSet map[int]int, withdrawalSet []int) State {
	newState := State{
		Epoch:   state.Epoch + 1,
		Balance: make(map[int]int),
		Sig:     make(map[int]string),
	}
	for k, v := range state.Balance {
		newState.Balance[k] = v
	}
	for k, v := range enrollmentSet {
		newState.Balance[k] = v
	}
	txCounter := make(map[int]int)
	nextLeaderCounter := 0
	nextLeaderId := state.LeaderId
	finishedTask = len(txSet)
	fmt.Printf("Epoch: %v, leader %v, finishTask: %v\n", state.Epoch, state.LeaderId, finishedTask)
	for _, tx := range txSet {
		if tx.Amount > newState.Balance[tx.Sender] || tx.Epoch != state.Epoch {
			fmt.Printf("calculateNextState error tx.Amount=%v, Sender.Balance=%v, tx.Epoch=%v, state.Epoch=%v\n",
				tx.Amount, newState.Balance[tx.Sender], tx.Epoch, state.Epoch)
			os.Exit(-1)
		}
		newState.Balance[tx.Sender] -= tx.Amount
		newState.Balance[tx.Receiver] += tx.Amount

		txCounter[tx.Sender]++
		if txCounter[tx.Sender] > nextLeaderCounter {
			nextLeaderCounter = txCounter[tx.Sender]
			nextLeaderId = tx.Sender
		}
		txCounter[tx.Receiver]++
		if txCounter[tx.Receiver] > nextLeaderCounter {
			nextLeaderCounter = txCounter[tx.Receiver]
			nextLeaderId = tx.Receiver
		}
	}
	//fmt.Printf("calculateNextState txCounter: %v\n", txCounter)
	newState.LeaderId = nextLeaderId
	for _, nodeId := range withdrawalSet {
		delete(newState.Balance, nodeId)
	}
	return newState
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
		//stat.txCount, float64(stat.txDuration)/float64(stat.txCount), 0,
		stat.txCount, Utils.Mean(stat.txLatency), Utils.StdDev(stat.txLatency),
	)
	stat.tradingPhaseMsgCount = 0
	stat.tradingPhaseDataSize = 0
	stat.consensusPhaseMsgCount = 0
	stat.consensusPhaseDataSize = 0
	stat.txCount = 0
	stat.txDuration = 0
	stat.txLatency = []int64{}
}

func (node *Node) leaderCron(ctx context.Context) {

	// 处理并展示上一个epoch的统计信息
	stat.consensusPhaseDuration = time.Since(stat.consensusPhaseStartTime)
	printAndClearStat(node.state.Epoch - 1)

	// 开始本周期
	stat.tradingPhaseStartTime = time.Now()

	for _, n := range nodesMap {
		go n.startTradingPhase()
	}

	for {
		select {
		case <-ctx.Done():
			node.leaderConsensusPhase()
			return
		case txIdRequest := <-node.txIdRequestChan:
			for i := 0; i < txIdRequest.num; i++ {
				txId := node.leader.txIdGenerator[txIdRequest.nodeId]
				node.leader.txIdGenerator[txIdRequest.nodeId]++
				txIdRequest.idList[i] = txId
				txIdRequest.sigList[i] = node.signTxid(txIdRequest.nodeId, txId)
			}
			go node.sendTxIdReply(ctx, txIdRequest.nodeId, txIdRequest)
		case tx := <-node.txLeaderChan:
			if tx.Epoch == node.state.Epoch {
				tx.duration = time.Since(tx.startTime)
				stat.txCount++
				stat.txLatency = append(stat.txLatency, tx.duration.Milliseconds())
				stat.txDuration += tx.duration.Milliseconds()
				node.leader.txSet = append(node.leader.txSet, tx)
			} else {
				fmt.Printf("leader %v phase %v, discarding tx from Epoch %v\n", node.id, node.state.Epoch, tx.Epoch)
			}
		}
	}
}

func (node *Node) leaderConsensusPhase() {

	stat.tradingPhaseDuration = time.Since(stat.tradingPhaseStartTime)
	stat.consensusPhaseStartTime = time.Now()

	for i := len(node.txLeaderChan); i > 0; i = len(node.txLeaderChan) {
		<-node.txLeaderChan
	}

	newState := calculateNextState(node.state, node.leader.txSet,
		node.leader.enrollmentSet, node.leader.withdrawalSet) //计算新状态
	newState.Sig[node.id] = node.sign(newState.Balance)
	newState.LeaderAddr = node.pubAddr
	printState(newState)
	node.verify(node.pubAddr, []byte(newState.Sig[node.id]), newState.Balance)

	for nodeId := range nodesMap {
		go node.sendNewState(context.TODO(), nodeId, newState)
	}
	var count = 0
	for i := 0; i < numNode; i++ {
		count++
		stateSig := <-node.newStateReplyChan
		//fmt.Printf("vote from Node %v\n",stateSig.NodeId)

		newState.Sig[stateSig.NodeId] = stateSig.Sig
		if count >= confNum {
			break
		}
	}
	for nodeId := range nodesMap {
		go node.sendNewStateConfirm(context.TODO(), nodeId, newState) //投票
	}
}

type TxIdSig struct {
	Epoch  int
	TxId   int
	Sender int
}

func (node *Node) signTxid(sender int, txid int) string {
	txIdSig := TxIdSig{Epoch: node.state.Epoch, TxId: txid, Sender: sender}
	return node.sign(txIdSig)
}

func (node *Node) startTradingPhase() {
	Utils.NetworkDelay(0)
	node.tradingPhaseChan <- struct{}{}
}

type TxIdRequest struct {
	nodeId  int
	num     int
	idList  []int
	sigList []string
}

func (node *Node) sendTxIdRequest(ctx context.Context, to int, txIdRequest TxIdRequest) {
	bytes := Utils.CountBytes(txIdRequest)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		//fmt.Printf("node %v Epoch %v sendTxIdRequest %v\n", node.id, node.state.Epoch, txIdRequest)
		stat.tradingPhaseMsgCount++
		stat.tradingPhaseDataSize += bytes
		nodesMap[to].txIdRequestChan <- txIdRequest
	}
}

func (node *Node) sendTxIdReply(ctx context.Context, to int, txIdRequest TxIdRequest) {
	bytes := Utils.CountBytes(txIdRequest)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		//fmt.Printf("node %v Epoch %v sendTxIdReply %v\n", node.id, node.state.Epoch, txIdRequest)
		stat.tradingPhaseMsgCount++
		stat.tradingPhaseDataSize += bytes
		nodesMap[to].txIdReplyChan <- txIdRequest
	}
}

func (node *Node) sendTxRequest(ctx context.Context, to int, tx Tx) {
	bytes := Utils.CountBytes(tx)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		//fmt.Printf("node %v Epoch %v sendTxRequest %v\n", node.id, node.state.Epoch, tx)
		stat.tradingPhaseMsgCount++
		stat.tradingPhaseDataSize += bytes
		nodesMap[to].txRequestChan <- tx
	}
}

func (node *Node) sendTxReply(ctx context.Context, to int, tx Tx) {
	bytes := Utils.CountBytes(tx)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		//fmt.Printf("node %v Epoch %v sendTxReply %v\n", node.id, node.state.Epoch, tx)
		stat.tradingPhaseMsgCount++
		stat.tradingPhaseDataSize += bytes
		nodesMap[to].txReplyChan <- tx
	}
}

func (node *Node) sendTxLeader(ctx context.Context, to int, tx Tx) {
	bytes := Utils.CountBytes(tx)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		//fmt.Printf("node %v Epoch %v sendTxLeader %v\n", node.id, node.state.Epoch, tx)
		stat.tradingPhaseMsgCount++
		stat.tradingPhaseDataSize += bytes
		nodesMap[to].txLeaderChan <- tx
	}
}

func (node *Node) sendNewState(ctx context.Context, to int, newState State) {
	bytes := Utils.CountBytes(newState)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		//fmt.Printf("node %v Epoch %v sendNewState %v\n", node.id, node.state.Epoch, newState)
		stat.consensusPhaseMsgCount++
		stat.consensusPhaseDataSize += bytes

		nodesMap[to].newStateChan <- newState
	}
}

func (node *Node) sendNewStateReply(ctx context.Context, to int, stateSig StateSig) {
	bytes := Utils.CountBytes(stateSig)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		//fmt.Printf("node %v Epoch %v sendNewStateReply %v\n", node.id, node.state.Epoch, stateSig)
		stat.consensusPhaseMsgCount++
		stat.consensusPhaseDataSize += bytes
		nodesMap[to].newStateReplyChan <- stateSig
	}
}

func (node *Node) sendNewStateConfirm(ctx context.Context, to int, newStateConfirm State) {
	bytes := Utils.CountBytes(newStateConfirm)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		//fmt.Printf("node %v Epoch %v sendNewStateConfirm %v\n", node.id, node.state.Epoch, newStateConfirm)
		stat.consensusPhaseMsgCount++
		stat.consensusPhaseDataSize += bytes
		nodesMap[to].newStateConfirmChan <- newStateConfirm
	}
}

func createNode(id int, balance int) *Node {
	key, err := crypto.GenerateKey()
	if err != nil {
		fmt.Println("createNode error: ", err.Error())
		os.Exit(1)
	}
	return &Node{
		id:                  id,
		balance:             balance,
		priKey:              key,
		pubAddr:             crypto.PubkeyToAddress(key.PublicKey).Hex(),
		txIdRequestChan:     make(chan TxIdRequest, targetTPS),
		txIdReplyChan:       make(chan TxIdRequest, targetTPS),
		txRequestChan:       make(chan Tx, targetTPS),
		txReplyChan:         make(chan Tx, targetTPS),
		txLeaderChan:        make(chan Tx, targetTPS),
		newStateChan:        make(chan State, 1),
		newStateReplyChan:   make(chan StateSig, numNode),
		newStateConfirmChan: make(chan State, 1),
		tradingPhaseChan:    make(chan struct{}, 1),
	}
}

func Run(n, e int) int {
	fmt.Println("Garou...")

	numNode = n
	initBalance = 10000
	tradingPhaseSecond = e

	targetTPS = 10000 //9000
	tpsPerNode := targetTPS / numNode
	tps := 0
	makeTaskChan = make(chan struct{}, 1)
	nodesMap = make(map[int]*Node)

	leaderId := rand.Intn(numNode)
	fmt.Println("initLeader: ", leaderId)

	initState := State{Epoch: 1, LeaderId: leaderId, Balance: make(map[int]int)}

	for i := 0; i < numNode; i++ {
		nodesMap[i] = createNode(i, initBalance)
		initState.Balance[i] = initBalance
	}

	for _, node := range nodesMap {
		go node.Run(initState)
	}

	for i := 0; i < 10; i++ {
		startTime := time.Now()
		for _, node := range nodesMap {
			node.taskChan = make(chan struct{}, targetTPS)
			go func(node *Node) {
				ticker := time.NewTicker(time.Duration(int(1000.0/float64(tpsPerNode))) * time.Millisecond)
				for i := 0; i < tpsPerNode*e; i++ {
					<-ticker.C
					node.taskChan <- struct{}{}
				}
			}(node)
		}
		<-makeTaskChan
		currentTps := int(float64(finishedTask) / float64(time.Since(startTime).Seconds()))
		if i >= 1 {
			tps += currentTps
		}
		fmt.Printf("\ntps: %d\n", int(float64(finishedTask)/float64(time.Since(startTime).Seconds())))
	}
	return tps / 10
}

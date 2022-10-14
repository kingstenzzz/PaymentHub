package TURBO

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
	merkletree "github.com/wealdtech/go-merkletree"
	"math"
	"math/rand"
	"os"
	"reflect"
	"sync"
	"time"
)

var numNode int
var initBalance int

var nodesMap map[int]*userNode

var finishedTask int
var makeTaskChan chan struct{}

var targetTPS int

var tradingPhaseSecond int

var confNum = 20

type Stat struct {
	tradingPhaseStartTime   time.Time
	tradingPhaseDuration    time.Duration
	consensusPhaseStartTime time.Time
	consensusPhaseDuration  time.Duration
	calculateDuration       time.Duration
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

type userNode struct {
	id      int
	balance int

	state     State
	leader    Leader
	validator bool
	priKey    *ecdsa.PrivateKey
	pubAddr   string

	txIdRequestChan chan TxIdRequest
	txIdReplyChan   chan TxIdRequest
	//txRequestChan   chan Tx
	//txReplyChan     chan Tx
	txLeaderChan chan Tx //发给leader

	newStateEpochChan      chan State
	newStateEpochReplyChan chan StateSig
	newStateConfirmChan    chan State

	tradingPhaseChan chan struct{}

	taskChan chan struct{}
}

type CheckPoint struct {
	Epoch             int
	LeaderId          int
	LeaderAddr        string
	PaymentRoot       []byte
	IntervalStateRoot []byte
	FinalStateRoot    []byte
	withdrawalRoot    []byte
	Sig               map[int]string
}

type Validator struct {
	txSet         []Tx
	enrollmentSet map[int]int
	withdrawalSet []int
	checkpoint    CheckPoint
}

type Leader struct {
	txSet         []Tx
	enrollmentSet map[int]int
	withdrawalSet []int
	txIdGenerator map[int]int
	checkpoint    CheckPoint
}

func (node *userNode) cleanUp() {
	for i := len(node.txIdRequestChan); i > 0; i = len(node.txIdRequestChan) {
		<-node.txIdRequestChan
	}
	for i := len(node.txIdReplyChan); i > 0; i = len(node.txIdReplyChan) {
		<-node.txIdReplyChan
	}
	/*
		for i := len(node.txRequestChan); i > 0; i = len(node.txRequestChan) {
			<-node.txRequestChan
		}
		for i := len(node.txReplyChan); i > 0; i = len(node.txReplyChan) {
			<-node.txReplyChan
		}
	*/

	for i := len(node.txLeaderChan); i > 0; i = len(node.txLeaderChan) {
		<-node.txLeaderChan
	}
}

func (node *userNode) Run(initState State) {
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
		if node.validator == true {
			//for {
			newState := <-node.newStateEpochChan
			stateSig := StateSig{
				Epoch:  newState.Epoch,
				NodeId: node.id,
				//Sig:    node.signNewState(newState),
				Sig: node.sign(newState),
			}
			//验证交易和签名Reply
			go node.sendNewStateEpochReply(context.TODO(), node.state.LeaderId, stateSig)
			//	}

		}
		// consensus phase

		newStateConfirm := <-node.newStateConfirmChan

		node.state = newStateConfirm
		node.balance = newStateConfirm.Balance[node.id]
		leaderId = newStateConfirm.LeaderId
		epoch = newStateConfirm.Epoch
	}
}

func (node *userNode) verify(pubkey string, sig []byte, msg interface{}) {
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

func (node *userNode) sign(msg interface{}) string {
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

func (node *userNode) getRandomNode() int {
	keys := reflect.ValueOf(nodesMap).MapKeys()
	for {
		nodeId := keys[rand.Intn(len(nodesMap))].Interface().(int)
		if nodeId != node.id {
			return nodeId
		}
	}
	return 0
}

func (node *userNode) getTxId(ctx context.Context) (int, string, error) {
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

func (node *userNode) senderTx(ctx context.Context, txid int, leaderSig string, txStartTime time.Time) {
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
	node.sendTxLeader(ctx, node.state.LeaderId, tx) //send to leader
	/*
		go node.sendTxRequest(ctx, tx.Receiver, tx)
		select {
		case <-ctx.Done():
			return
		//case txReply := <-node.txReplyChan:
			node.sendTxLeader(ctx, node.state.LeaderId, txReply)
		}
	*/

}

func (node *userNode) receiverTx(ctx context.Context, tx Tx) {
	tx.ReceiverSig = node.sign(tx)
	node.sendTxReply(ctx, tx.Sender, tx)
}

func (node *userNode) initLeader(ctx context.Context, leaderId int, epochId int) {
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
	Epoch         int
	LeaderId      int
	LeaderAddr    string
	Balance       map[int]int
	Sig           map[int]string
	enrollmentSet map[int]int
	withdrawalSet []int
	txSet         []Tx
	checkpoint    CheckPoint
}

type StateSig struct {
	Epoch  int
	NodeId int
	Sig    string
}

func printState(state State) {
	fmt.Printf("Epoch: %v, LeaderId: %v, Balance: %v",
		state.Epoch, state.LeaderId, state.Balance)
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
	SRL := make([][]byte, finishedTask+1)
	balanceByte := make([][]byte, numNode)
	txSetByte := make([][]byte, finishedTask)
	BatchBalanceByte := make([][][]byte, finishedTask)
	for i, balance := range newState.Balance {
		balancebyte, _ := Utils.Encode(balance)
		balanceByte[i] = balancebyte
	}
	SR1tree, _ := merkletree.New(balanceByte)
	SR1 := SR1tree.Root()
	SRL[0] = SR1

	for i, tx := range txSet {
		if tx.Amount > newState.Balance[tx.Sender] || tx.Epoch != state.Epoch {
			//fmt.Printf("calculateNextState error tx.Amount=%v, Sender.Balance=%v, tx.Epoch=%v, state.Epoch=%v\n",
			//tx.Amount, newState.Balance[tx.Sender], tx.Epoch, state.Epoch)
			os.Exit(-1)
		}

		newState.Balance[tx.Sender] -= tx.Amount
		balancebyte, _ := Utils.Encode(newState.Balance[tx.Sender])
		balanceByte[tx.Sender] = balancebyte
		newState.Balance[tx.Receiver] += tx.Amount
		balancebyte, _ = Utils.Encode(newState.Balance[tx.Receiver])
		balanceByte[tx.Receiver] = balancebyte
		//交易最多的多leader？
		txCounter[tx.Sender]++
		if txCounter[tx.Sender] > nextLeaderCounter {
			nextLeaderCounter = txCounter[tx.Sender]
			nextLeaderId = tx.Sender
		}
		txbyte, _ := Utils.Encode(tx)
		txSetByte = append(txSetByte, txbyte)

		//txCounter[tx.Receiver]++
		//if txCounter[tx.Receiver] > nextLeaderCounter {
		//	nextLeaderCounter = txCounter[tx.Receiver]
		//	nextLeaderId = tx.Receiver
		//}

		/*
			for i, balance := range newState.Balance {
				balancebyte, _ := Utils.Encode(balance)
				balanceByte[i] =balancebyte
			}
		*/
		BatchBalanceByte[i] = balanceByte

	}

	var wg sync.WaitGroup //定义一个同步等待的组

	cpuNum := 24
	fmt.Printf("cpu:%v", cpuNum)

	for i := 1; i < cpuNum; i++ {
		wg.Add(1)
		go func() {
			for j := (finishedTask * (i - 1) / cpuNum) + 1; j < finishedTask*(i/cpuNum); j++ {
				SR2tree, _ := merkletree.New(BatchBalanceByte[j])
				SR2 := SR2tree.Root()
				SRL[j] = SR2
			}
			wg.Done()

		}()

	}

	wg.Wait() //阻塞直到所有任务完成
	fmt.Println("over\n")
	ISRL := make([][]byte, finishedTask)
	for i := 0; i < finishedTask-1; i++ {
		IS := sha256.New()
		IS.Write(SRL[i])
		IS.Sum(SRL[i+1])
		ISByte := IS.Sum(txSetByte[i])
		ISRL[i] = ISByte //添加过程根
	}
	ISRtree, _ := merkletree.New(ISRL)
	txSetTree, _ := merkletree.New(txSetByte)
	if finishedTask != 0 {
		newState.checkpoint.PaymentRoot = txSetTree.Root()
		newState.checkpoint.FinalStateRoot = SRL[finishedTask-1]
		newState.checkpoint.IntervalStateRoot = ISRtree.Root()

		newState.checkpoint.LeaderId = newState.LeaderId
		newState.checkpoint.Epoch = newState.Epoch
		fmt.Printf("paymentRoot%x\n，ISR%x\n,StateRoot%x\n", newState.checkpoint.PaymentRoot, newState.checkpoint.IntervalStateRoot, newState.checkpoint.FinalStateRoot)

		newState.LeaderId = nextLeaderId
		for _, nodeId := range withdrawalSet {
			delete(newState.Balance, nodeId)
		}
		newState.txSet = txSet
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

func (node *userNode) leaderCron(ctx context.Context) {

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
				fmt.Printf("leader %v phase %v, discarding tx from Epoch %v\n", node.id, node.state.Epoch, tx.Epoch) //不是这轮的交易
			}
		}
	}
}

func (node *userNode) leaderConsensusPhase() {

	stat.tradingPhaseDuration = time.Since(stat.tradingPhaseStartTime)
	stat.consensusPhaseStartTime = time.Now()

	for i := len(node.txLeaderChan); i > 0; i = len(node.txLeaderChan) {
		<-node.txLeaderChan
	}
	if node.leader.txSet != nil {
		calculiStetTime := time.Now()
		newState := calculateNextState(node.state, node.leader.txSet,
			node.leader.enrollmentSet, node.leader.withdrawalSet)
		stat.calculateDuration = time.Since(calculiStetTime)
		newState.Sig[node.id] = node.sign(newState.Balance)
		newState.LeaderAddr = node.pubAddr
		//printState(newState)
		//for _,tx :=range node.leader.txSet{
		//	fmt.Printf("id:%v,sender:%v.receiver:%v\n",tx.Id,tx.Sender,tx.Receiver)
		//}
		//node.verify(node.pubAddr, []byte(newState.Sig[node.id]),newState.Balance)

		for nodeId := range nodesMap { //改成验证者node
			if nodesMap[nodeId].validator {
				go node.sendNewEpochState(context.TODO(), nodeId, newState)
			}
		}
		var count = 0
		for i := 0; i < numNode; i++ {
			count++
			stateSig := <-node.newStateEpochReplyChan
			//fmt.Printf("vote from userNode %v\n", stateSig.NodeId)
			newState.Sig[stateSig.NodeId] = stateSig.Sig
			if count >= confNum {
				break
			}
		}
		for nodeId := range nodesMap {
			//if nodesMap[nodeId].validator {
			go node.sendNewStateConfirm(context.TODO(), nodeId, newState)

			//}
		}
	}

}

type TxIdSig struct {
	Epoch  int
	TxId   int
	Sender int
}

func (node *userNode) signTxid(sender int, txid int) string {
	txIdSig := TxIdSig{Epoch: node.state.Epoch, TxId: txid, Sender: sender}
	return node.sign(txIdSig)
}

func (node *userNode) startTradingPhase() {
	Utils.NetworkDelay(0)
	node.tradingPhaseChan <- struct{}{}
}

type TxIdRequest struct {
	nodeId  int
	num     int
	idList  []int
	sigList []string
}

func (node *userNode) sendTxIdRequest(ctx context.Context, to int, txIdRequest TxIdRequest) {
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

func (node *userNode) sendTxIdReply(ctx context.Context, to int, txIdRequest TxIdRequest) {
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

func (node *userNode) sendTxRequest(ctx context.Context, to int, tx Tx) {
	bytes := Utils.CountBytes(tx)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		//fmt.Printf("node %v Epoch %v sendTxRequest %v\n", node.id, node.state.Epoch, tx)
		stat.tradingPhaseMsgCount++
		stat.tradingPhaseDataSize += bytes
		//nodesMap[to].txRequestChan <- tx
	}
}

func (node *userNode) sendTxReply(ctx context.Context, to int, tx Tx) {
	bytes := Utils.CountBytes(tx)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		//fmt.Printf("node %v Epoch %v sendTxReply %v\n", node.id, node.state.Epoch, tx)
		stat.tradingPhaseMsgCount++
		stat.tradingPhaseDataSize += bytes
		//nodesMap[to].txReplyChan <- tx
	}
}

func (node *userNode) sendTxLeader(ctx context.Context, to int, tx Tx) {
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

func (node *userNode) sendNewEpochState(ctx context.Context, to int, newState State) {
	bytes := Utils.CountBytes(newState)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		//fmt.Printf("node %v Epoch %v sendNewState %v\n", node.id, node.state.Epoch, newState)
		stat.consensusPhaseMsgCount++
		stat.consensusPhaseDataSize += bytes

		nodesMap[to].newStateEpochChan <- newState
	}
}

func (node *userNode) sendNewStateEpochReply(ctx context.Context, to int, stateSig StateSig) {
	bytes := Utils.CountBytes(stateSig)
	Utils.NetworkDelay(bytes)
	select {
	case <-ctx.Done():
		return
	default:
		//fmt.Printf("node %v Epoch %v sendNewStateEpochReply %v\n", node.id, node.state.Epoch, stateSig)
		stat.consensusPhaseMsgCount++
		stat.consensusPhaseDataSize += bytes
		nodesMap[to].newStateEpochReplyChan <- stateSig
	}
}

func (node *userNode) sendNewStateConfirm(ctx context.Context, to int, newStateConfirm State) {
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

func createNode(id int, balance int, validator bool) *userNode {
	key, err := crypto.GenerateKey()
	if err != nil {
		fmt.Println("createNode error: ", err.Error())
		os.Exit(1)
	}
	return &userNode{
		id:              id,
		balance:         balance,
		priKey:          key,
		pubAddr:         crypto.PubkeyToAddress(key.PublicKey).Hex(),
		txIdRequestChan: make(chan TxIdRequest, targetTPS),
		txIdReplyChan:   make(chan TxIdRequest, targetTPS),
		//txRequestChan:       make(chan Tx, targetTPS),
		//txReplyChan:         make(chan Tx, targetTPS),
		txLeaderChan:           make(chan Tx, targetTPS),
		newStateEpochChan:      make(chan State, 1),
		newStateEpochReplyChan: make(chan StateSig, numNode),
		newStateConfirmChan:    make(chan State, 1),
		tradingPhaseChan:       make(chan struct{}, 1),
		validator:              validator,
	}
}

func Run(n, e, v int) int {
	fmt.Println("Turbo...")

	numNode = n
	initBalance = 20000
	tradingPhaseSecond = e
	targetTPS = 10000 //9000
	tpsPerNode := targetTPS / numNode
	tps := 0
	confNum = v
	var consensDuration time.Duration = time.Nanosecond
	var epochTime float64
	makeTaskChan = make(chan struct{}, 1)
	nodesMap = make(map[int]*userNode)

	leaderId := rand.Intn(numNode) //根据月选举
	fmt.Println("initLeader: ", leaderId)
	fmt.Println("vNode: ", confNum)

	initState := State{Epoch: 1, LeaderId: leaderId, Balance: make(map[int]int)}

	for i := 0; i < numNode; i++ {
		if i <= confNum {
			nodesMap[i] = createNode(i, initBalance, true)
		} else {
			nodesMap[i] = createNode(i, initBalance, false)

		}

		initState.Balance[i] = initBalance
	}

	for _, node := range nodesMap {
		go node.Run(initState)
	}

	for i := 0; i < 11; i++ {
		startTime := time.Now()
		for _, node := range nodesMap {
			node.taskChan = make(chan struct{}, targetTPS)
			go func(node *userNode) {
				ticker := time.NewTicker(time.Duration(int(1000.0/float64(tpsPerNode))) * time.Millisecond)
				for i := 0; i < tpsPerNode*e; i++ {
					<-ticker.C
					node.taskChan <- struct{}{}
				}
			}(node)

		}
		<-makeTaskChan
		epochTime = float64(time.Since(startTime).Seconds())
		currentTps := int(float64(finishedTask) / float64(time.Since(startTime).Seconds()))
		if i >= 1 {
			tps += currentTps
		}
		if i >= 1 {
			fmt.Println("consensusPhaseDuration：", stat.consensusPhaseDuration)
			consensDuration += stat.consensusPhaseDuration

		}
		fmt.Printf("\ntps: %d\n", int(float64(finishedTask)/float64(time.Since(startTime).Seconds())))
	}
	fmt.Printf("atps=%d acd=%v epochTime=%v calculateDuration:%v tradingPhaseDuration:%v\n",
		tps/10, float64(consensDuration)/math.Pow(10, 10), epochTime, stat.calculateDuration, stat.tradingPhaseDuration)
	return tps / 10
}

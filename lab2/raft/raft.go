package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"
	// "fmt"
	"sync"
	"sync/atomic"

	// "testing/iotest"
	"math/rand"
	"time"

	//	"6.824/labgob"
	"6.824/labrpc"
)

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

type LogEntry struct{
	lterm int
	command interface{}
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	//所有机器持久化状态
	currentTerm 		int			//server 存储的最新任期（初始化为 0 且单调递增）
	voteFor				int			//当前任期接受到的选票的候选者 ID（初值为 null）
	log 				[]LogEntry	//日志记录每条日记记录包含状态机的命令和从 leader 接受到日志的任期。(索引初始化为 1)
	logLock				sync.Mutex
	//所有机器的可变状态
	commitIndex 		int			//将被提交的日志记录的索引（初值为 0 且单调递增）
	lastApplied			int	 		//已经被提交到状态机的最后一个日志的索引（初值为0 且单调递增）
	//leader 的可变状态：
	// nextIndex[] 		//每台机器在数组占据一个元素，元素的值为下条发送到该机器的日志索引 (初始值为 leader 最新一条日志的索引 +1)
	// matchIndex[] 		//每台机器在数组中占据一个元素，元素的记录将要复制给该机器日志的索引的。

	//当前状态
	state int
	// stateLock sync.Locker
	//定时器
	timer	*time.Timer
	timerLock	sync.Mutex
	
	//是否开启选举
	electionEvent bool
	electionEventLocker sync.Mutex
}

const (
	Follower = iota
	Candidate
	Leader
)

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term = rf.currentTerm
	isleader = rf.state == Leader
	return term, isleader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}


//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}


//
// A service wants to switch to snapshot.  Only do so if Raft hasn't
// have more recent info since it communicate the snapshot on applyCh.
//
func (rf *Raft) CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool {

	// Your code here (2D).

	return true
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).

}

//-------------------------- myRPCstart

type AppendEntrisReq struct{
	Term		int	//当前任期
	LeaderId	int //领导者ID
	TestCnt		int
}

type AppendEntrisResp struct{
	Term	int		//当前任期
	Success	bool	//
}

func (rf *Raft) AppendEntris(req *AppendEntrisReq, resp *AppendEntrisResp){
	// fmt.Printf("%d -> %d AppendEntrus, cnt : %d, term = %d\n", req.LeaderId, rf.me, req.TestCnt, req.Term)
	//现在处理leader发送过来的心跳请求
	rf.mu.Lock()
	defer rf.mu.Unlock()
	resp.Term = rf.currentTerm
	if(req.Term >= rf.currentTerm){
		resp.Success = true
		rf.state = Follower
		rf.currentTerm = req.Term
		rf.updateELectionEvent(true)
	}else{
		resp.Success = false
	}
}

func (rf *Raft) sendRequestAppendEntris(server int, args *AppendEntrisReq, reply *AppendEntrisResp) bool {
	ok := rf.peers[server].Call("Raft.AppendEntris", args, reply)
	return ok
}

//-------------------------- myRPCend


//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term			int	//候选者的任期
	CandidateId		int	//候选者编号
	LastLogIndex	int	//候选者最后一条日志记录的索引
	LastLogTerm		int	//候选者最后一条日志的索引的任期
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term		int	//当前任期，候选者用来更新自己
	VoteGranted	bool//候选者当选就true
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Term = rf.currentTerm
	// fmt.Printf("%d -> %d request RPC vote in term = %d\ncurrentTerm = %d\n", args.CandidateId, rf.me, args.Term, rf.currentTerm)
	if(args.Term > rf.currentTerm){
		//投票
		// if rf.voteFor == -1{
		// 	reply.VoteGranted = true
		// 	rf.voteFor = args.CandidateId
		// }else{
		// 	reply.VoteGranted = false
		// }
		reply.VoteGranted = true;
		//更新状态	
		rf.currentTerm = args.Term
		rf.state = Follower
		rf.voteFor = args.CandidateId
		rf.updateELectionEvent(true)
	} else if(args.Term == rf.currentTerm){
		
	} else {
		//不投票
		reply.VoteGranted = false
	}
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}


//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).
	term,isLeader = rf.GetState()
	if(!isLeader){
		return index, term, isLeader
	}

	println(len(rf.log))
	rf.mu.Lock()
		rf.log = append(rf.log, LogEntry{term, command})
	rf.mu.Unlock()
	


	return index, term, isLeader
}

//
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// The ticker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
func (rf *Raft) ticker() {
	for rf.killed() == false {

		

		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using
		// time.Sleep().
		// nowterm,isLeader := rf.GetState()
		// fmt.Println(rf.me, " ticker is running, term =", nowterm, "leader:", isLeader)
		_,isLeader := rf.GetState()
		if !isLeader{
			timeout := generateRandomElectionTimeout()
			time.Sleep(150 * time.Millisecond)
			electionEvent := rf.getElectionEvent()
			if !electionEvent{
				time.Sleep(timeout)
				electionEvent1 := rf.getElectionEvent()
				if !electionEvent1{
					go rf.startElection()
				}
			}
			rf.updateELectionEvent(false)
		}else{
			// nowterm,_ := rf.GetState()
			// fmt.Printf("leader : %d, term = %d\n", rf.me, nowterm)
			go rf.startHeartBeat()
			time.Sleep(110 * time.Millisecond)
		}

	}
}

//------------------------------- mycode start

var electionTimeoutMin int = 250
var electionTimeoutMax int = 500
func generateRandomElectionTimeout() time.Duration {
	rand.Seed(time.Now().UnixNano())
	randomTimeout := time.Duration(rand.Intn(electionTimeoutMax - electionTimeoutMin) + electionTimeoutMin)
	return randomTimeout * time.Millisecond
}
//更新选举超时定时器
func(rf *Raft)updateElectionTimeout(){
	rf.timerLock.Lock()
	defer rf.timerLock.Unlock()
	timeout := generateRandomElectionTimeout()
	rf.timer.Reset(timeout)
}
//更新选举变量
func(rf *Raft)updateELectionEvent(hasElectionEvent bool){
	rf.electionEventLocker.Lock()
	defer rf.electionEventLocker.Unlock()
	rf.electionEvent = hasElectionEvent
}
func(rf *Raft)getElectionEvent()(electionEvent bool){
	rf.electionEventLocker.Lock()
	defer rf.electionEventLocker.Unlock()
	return rf.electionEvent 
}

//开始新的选举
func(rf *Raft)startElection(){
	// nowterm,_ := rf.GetState()
	// fmt.Println(rf.me, "startElection, term =", nowterm)
	//节点数量
	var nodeSum int = len(rf.peers)
	//得票
	// var voteCnt int = 0;
	// var voteLocker sync.Mutex
	
	ch := make(chan bool, 2)
	count := 0

	// vote_cnt_incress := func(){
	// 	voteLocker.Lock()
	// 	defer voteLocker.Unlock()
	// 	voteCnt++
	// }

	

	//先投给自己
	rf.mu.Lock()
		rf.currentTerm++
		if(rf.state == Follower){
			rf.state = Candidate
		}
		rf.voteFor = rf.me
		ch <- true
		// vote_cnt_incress()
		// fmt.Println(rf.me, "now is ok")
	rf.mu.Unlock()
	rf.updateELectionEvent(true)
	for i := 0; i < nodeSum; i++{
		if i != rf.me{
			go func(peerIndex int){
				rf.mu.Lock()
				nowState := rf.state
				// nowterm1 := rf.currentTerm
				logLen := len(rf.log)
				args := &RequestVoteArgs{
					Term: 			rf.currentTerm,
					CandidateId: 	rf.me,
					LastLogIndex: 	logLen-1,
					LastLogTerm: 	rf.log[logLen-1].lterm,
				}
				reply := &RequestVoteReply{}
				rf.mu.Unlock()
				//只有C才可以选举
				if(nowState != Candidate){
					
					// fmt.Printf("%d now is not Candidate, term = %d\n", rf.me, nowterm1)
					return
				}
				ok := rf.sendRequestVote(peerIndex, args, reply)
				if ok{
					ch <- reply.VoteGranted
				}else{
					// fmt.Printf("%d to %d RPCVote is failed, term = %d\n", rf.me, peerIndex, nowterm1)
					ch <- false
				}
				
				// if reply.VoteGranted{
				// 	vote_cnt_incress()
				// }
				
			}(i)
		}
	}

	rf.mu.Lock()
		nowState := rf.state
	rf.mu.Unlock()
	// nowterm,_ = rf.GetState()
	
	if(nowState != Candidate){
		// fmt.Printf("%d now is not Candidate, term = %d\n", rf.me, nowterm)
		return
	}

	for i := 0; i < nodeSum; i++{
		v := <- ch
		if(v){
			count++
		}
		if(count > nodeSum / 2) {
			break;
		}
	}

	// nowterm,_ = rf.GetState()

	// fmt.Printf("%d : count = %d, term = %d\n", rf.me, count, nowterm)

	//选举成功
	if count > nodeSum / 2{
		rf.changToLeader()
	}
}

var ttCnt = 0
var ttLock sync.Mutex
func ttIncrese(){
	ttLock.Lock()
	defer ttLock.Unlock()
	ttCnt++
}
func getTTCnt() int{
	ttLock.Lock()
	defer ttLock.Unlock()
	return ttCnt
}
//开始一轮心跳
func(rf *Raft)startHeartBeat(){
	_, is_leader := rf.GetState();
	if(!is_leader) {
		return
	}
	ttIncrese()
	tCnt := getTTCnt()
	//理论上在这里需要等待所有的节点结果，但是我们简单一点，先试试测试
	var nodeSum int = len(rf.peers)
	for i := 0; i < nodeSum; i++{
		if(i != rf.me){
			go func(peerIndex int){
				now_term, is_leader := rf.GetState();
				if(!is_leader) {
					return
				}
				rf.mu.Lock()
					args := &AppendEntrisReq{
						Term: rf.currentTerm,
						LeaderId: rf.me,
						TestCnt: tCnt,
					}
					reply := &AppendEntrisResp{}
				rf.mu.Unlock()
				rf.sendRequestAppendEntris(peerIndex, args, reply)
				if(reply.Term > now_term){
					rf.changeToFollwer(now_term)
					return;
				}
			}(i)
		}
	}
}

func (rf *Raft)changToLeader(){
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.state = Leader
	// rf.electionEvent = true
	rf.updateELectionEvent(true)
}

func(rf *Raft) changeToFollwer(now_term int){
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.state = Follower
	rf.currentTerm = now_term
	rf.updateELectionEvent(false)
}

//------------------------------- mycode end

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.state = Follower
	rf.currentTerm  = 0
	rf.voteFor = -1
	rf.log = make([]LogEntry, 1)
	// rf.timer = time.NewTimer(0)
	rf.electionEvent = false
		
	// fmt.Printf("%d make is ok\n", rf.me)
	
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()


	return rf
}

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
	"labrpc"
	"math/rand"
	"sync"
	"time"
)

// import "bytes"
// import "labgob"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

type LogEntry struct {
	LogIndex, LogTerm int
	Command           interface{}
}

const (
	TIMEOUT_ELECTION_BASE = 800
	TIMEOUT_HB            = 200
)

type State int

const (
	STATE_FOLL State = iota
	STATE_CAND
	STATE_LEAD
)

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]

	isLeader   bool
	state      State
	totalVote  int
	stepdown   chan int
	electClock *time.Timer
	voters     map[int]bool

	// Persistent on ALL
	currentTerm int
	votedFor    int
	log         []LogEntry

	// Volatile on ALL
	commitIndex int
	lastApplied int

	// Volatile on LEADER
	nextIndex  []int
	matchIndex []int
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool

	term = rf.currentTerm
	isleader = rf.isLeader

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
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

type AppendEntriesArgs struct {
	Term, LeaderId, PrevLogIndex, PrevLogTerm int
	Entries                                   []LogEntry
	LeaderCommit                              int
}

type AppendEntriesReply struct {
	Term    int
	Success bool
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	reply.VoteGranted = false
	reply.Term = rf.currentTerm

	if args.Term >= rf.currentTerm && (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && rf.isMoreUpd(args.LastLogIndex, args.LastLogTerm) {
		reply.VoteGranted = true
	}

}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	reply.Term = rf.currentTerm

	// Check term
	if args.Term < rf.currentTerm {
		reply.Success = false
		return
	}

	// Check last index
	_idx, _term := args.PrevLogIndex, args.PrevLogTerm
	if len(rf.log)-1 < _idx || rf.log[_idx].LogTerm != _term {
		reply.Success = false
		return
	}

	// Append logs
	rf.log = append(rf.log[:_idx+1], args.Entries[:]...)

	if args.LeaderCommit > rf.commitIndex {
		if args.LeaderCommit > len(rf.log)-1 {
			rf.commitIndex = len(rf.log) - 1
		} else {
			rf.commitIndex = args.LeaderCommit
		}
	}

}

func (rf *Raft) prepareRequestVoteArgs() *RequestVoteArgs {
	var args *RequestVoteArgs

	args = &RequestVoteArgs{}

	rf.mu.Lock()
	args.CandidateId = rf.me
	lastLog := rf.log[len(rf.log)-1]
	args.LastLogTerm = lastLog.LogTerm
	args.LastLogIndex = lastLog.LogIndex
	rf.mu.Unlock()

	return args
}

func (rf *Raft) prepareAppendEntriesArg(peer int, lastLog LogEntry) (*AppendEntriesArgs, bool) {

	//TODO
	entries := []LogEntry(nil)

	//TODO: if up to date
	send := true

	args := &AppendEntriesArgs{
		rf.currentTerm,
		rf.me,
		lastLog.LogIndex,
		lastLog.LogTerm,
		entries,
		rf.commitIndex,
	}

	return args, send
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

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
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

	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
}

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
	rf.isLeader = false
	rf.lastApplied = 0
	rf.commitIndex = 0

	rf.currentTerm = 0
	rf.votedFor = -1
	rf.log = make([]LogEntry, 1)
	rf.stepdown = make(chan int)
	rf.state = STATE_FOLL

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// requestVote timers
	timeout := (time.Duration)(TIMEOUT_ELECTION_BASE+rand.Intn(150)) * time.Millisecond
	rf.electClock = time.AfterFunc(timeout, rf.doRequestVote)

	return rf
}

func (rf *Raft) isMoreUpd(idx, term int) bool {
	lastLog := rf.log[len(rf.log)-1]

	if lastLog.LogTerm > term {
		return false
	}

	if lastLog.LogTerm == term && len(rf.log)-1 > idx {
		return false
	}

	return true
}

// Request vote to all the peers
func (rf *Raft) doRequestVote() {
	votes := make(chan int)
	args := rf.prepareRequestVoteArgs()

	rf.upTerm()
	rf.totalVote = 0
	rf.voters = make(map[int]bool)
	rf.changeState(STATE_CAND)
	rf.upVote()

	// INV: Clock must be stopped ?
	rf.electClock.Reset(randTimeout())

	// Send requestVote parrallel
	for to, _ := range rf.peers {
		go func(server int) {
			reply := &RequestVoteReply{}
			ok := rf.sendRequestVote(server, args, reply)
			if ok {
				if reply.Term > rf.currentTerm {
					rf.stepDown(reply.Term)
				}

				if reply.VoteGranted && !rf.voters[server] {
					votes <- 1
					rf.voters[server] = true
				}
			}
		}(to)
	}

	// Handle responses
	for {
		select {
		case _ = <-votes:
			rf.upVote()
			if rf.isElected() {
				rf.doLead()
				return
			}
		case newTerm := <-rf.stepdown:
			// Change to Follow
			rf.stepDown(newTerm)
			return
		case <-rf.electClock.C:
			// Restart election
			rf.electClock.Reset(randTimeout())
			return
		}
	}
}
func (rf *Raft) upVote() {
	rf.totalVote += 1
}

func (rf *Raft) isElected() bool {
	return (len(rf.peers)+1)/2 <= rf.totalVote
}

func (rf *Raft) getLastLog() LogEntry {
	return rf.log[len(rf.log)-1]
}

func (rf *Raft) doAppendEntries() {
	lastLog := rf.getLastLog()
	for to, _ := range rf.peers {
		if !rf.isLeader {
			return
		}

		go func(server int) {
			reply := &AppendEntriesReply{}
			args, send := rf.prepareAppendEntriesArg(server, lastLog)
			if !send || !rf.isLeader {
				// Up to date
				return
			}
			ok := rf.sendAppendEntries(server, args, reply)

			if ok {
				if reply.Success {
					// Update
					rf.nextIndex[server] = lastLog.LogIndex + 1
					rf.matchIndex[server] = lastLog.LogIndex
				} else {
					if reply.Term > rf.currentTerm {
						rf.stepdown <- reply.Term
					} else {
						rf.nextIndex[server] -= 1
					}
				}
			}
		}(to)
	}
}

func (rf *Raft) doLead() {
	// Stop the elect timer
	if !rf.electClock.Stop() {
		<-rf.electClock.C
	}

	rf.isLeader = true
	rf.changeState(STATE_LEAD)

	// Start the heartbeat
	ticker := time.NewTimer(TIMEOUT_HB * time.Millisecond)

	go func() {
		for {
			select {
			case <-ticker.C:
				rf.doAppendEntries()
			case newTerm := <-rf.stepdown:
				ticker.Stop()
				rf.stepDown(newTerm)
				return
			}
		}
	}()
}

func (rf *Raft) stepDown(newTerm int) {

	rf.mu.Lock()
	rf.currentTerm = newTerm
	rf.isLeader = false
	rf.mu.Unlock()

	rf.persist()
	rf.changeState(STATE_FOLL)

	rf.electClock.Reset(randTimeout())
}

func (rf *Raft) changeState(state State) {
	rf.state = state
}

func (rf *Raft) upTerm() {
	rf.mu.Lock()
	rf.currentTerm += 1
	rf.mu.Unlock()

	rf.persist()
}

func randTimeout() time.Duration {
	return (time.Duration)(TIMEOUT_ELECTION_BASE+rand.Intn(150)) * time.Millisecond
}

package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	// "internal/itoa"
	"os"
	"strconv"
)

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.

type Task struct{
	Type	int
	Id		int
	FileName	string
	StartTime	string
	Reduce_cnt  int
}

type TaskReq struct{

}

type TaskResp struct{
	// Worker_id int
	Woker_Task Task
}

type MapTaskReq struct{
	Task_Id int
}
type MapTaskResp struct{

}

type ReduceTaskReq struct{
	Task_Id int
}
type ReduceTaskResp struct{

}

// type CompleteWorkerArgs struct{}
// type CompleteWorkerReply struct{}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}

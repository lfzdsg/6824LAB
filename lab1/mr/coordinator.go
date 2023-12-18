package mr

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"time"
	"container/list"
	"sync"
	"errors"
)


type Coordinator struct {
	// Your definitions here.
	state int
	state_lock	sync.Mutex

	free_maper_id 	*Queue			//没被使用的map worker Id
	free_reducer_id 	*Queue		//没被使用的reduce worker Id
	id_lock	sync.Mutex

	free_maper_task *Queue			//map任务队列
	free_reducer_task *Queue		//reduce任务队列
	task_lock	sync.Mutex	//GetTask lock

	map_task_state	map[int]int 	//1 ready 2 running 3 finish 0 not
	reduce_task_state map[int]int	//1 ready 2 running 3 finish 0 not
	
	map_finish_cnt	int				//map 完成数量
	reduce_finish_cnt	int			//reduce 完成数量
	finished_lock	sync.Mutex

	timer_lock	sync.Mutex

	// map_task	   	chan *Task
	// reduce_task		chan *Task
	map_cnt	int
	reduce_cnt	int
	cnt_lock sync.Mutex

	map_ok	bool
	reduce_ok	bool
	ok_lock	sync.Mutex
}

const (
	Coor_Start = iota
	Map_Task
	Reduce_Task
	WaitMap
	WaitReduce
	Success
)

//queue start
type Queue struct{
	queue list.List
	// queue_lock sync.Mutex
}
func (q *Queue)push_back(node interface{}){
	// q.queue_lock.Lock()
	// defer q.queue_lock.Unlock()
	q.queue.PushBack(node)
}
func (q *Queue)pop() error{
	// q.queue_lock.Lock()
	// defer q.queue_lock.Unlock()
	if(q.queue.Len() == 0){
		return errors.New("pop : the size of queue is zero")
	}
	q.queue.Remove(q.queue.Front())
	return nil
}
func (q *Queue)top() (interface{}, error){
	// q.queue_lock.Lock()
	// defer q.queue_lock.Unlock()
	if(q.queue.Len() == 0){
		return nil,errors.New("top : the size of queue is zero")
	}
	return q.queue.Front().Value,nil
}
func (q *Queue)empty()bool{
	// q.queue_lock.Lock()
	// defer q.queue_lock.Unlock()
	return q.queue.Len() == 0
}
func (q *Queue)len()int{
	// q.queue_lock.Lock()
	// defer q.queue_lock.Unlock()
	return q.queue.Len()
}
func construct_Queue() *Queue{
	return &Queue{}
}
//queue end
func(c *Coordinator) getState() int{
	c.state_lock.Lock()
	now_state := c.state
	c.state_lock.Unlock()
	return now_state
}


// Your code here -- RPC handlers for the worker to call.

func(c *Coordinator) GetTask(req *TaskReq, resp *TaskResp)error{
	// fmt.Println("GetTask is use")
	c.task_lock.Lock()
	defer c.task_lock.Unlock()

	now_state := c.getState()
	
	switch(now_state){
	case Map_Task:
		// fmt.Println("map_task, len =", c.free_maper_task.len())
		//任务队列中没有任务
		if(c.free_maper_task.len() == 0){
			fmt.Println("map len is zero")
			// resp.Worker_id = -1
			resp.Woker_Task = Task{
				Type: WaitMap,
				Id: -1,
				FileName: "",
				StartTime: "",
			}
			return nil
		}

		//取任务
		task,err := c.free_maper_task.top()
		if(err != nil){
			return err
		}
		var now_task = task.(Task)

		c.finished_lock.Lock()
		if(c.map_task_state[now_task.Id] == 0 || c.map_task_state[now_task.Id] == 3){
			defer c.finished_lock.Unlock()
			//不存在 | 做完了
			resp.Woker_Task = Task{
				Type: WaitMap,
				Id: -1,
				FileName: "",
				StartTime: "",
			}
			err = c.free_maper_task.pop()
			
			if(err != nil){
				return errors.New("MapTask, none or finished")
			}
			return nil
		}
		c.finished_lock.Unlock()

		//ready || running
		resp.Woker_Task = now_task
		resp.Woker_Task.StartTime =  time.Now().Format("2006-01-02 15:04:05")
		err = c.free_maper_task.pop()

		if(err != nil){
			return errors.New("MapTask: pop fail, len is zero")
			// return err
		}

		//将任务的值修改为运行中
		c.finished_lock.Lock()
		c.map_task_state[now_task.Id] = 2
		c.finished_lock.Unlock()
		
	
		return nil
	
	case Reduce_Task:
		// fmt.Println("reduce_task, len =", c.free_reducer_task.len())
		//任务队列中没有任务
		if(c.free_reducer_task.len() == 0){
			// resp.Worker_id = -1
			resp.Woker_Task = Task{
				Type: WaitReduce,
				Id: -1,
				FileName: "",
				StartTime: "",
			}
			return nil
		}

		//取任务
		task,err := c.free_reducer_task.top()
		if(err != nil){
			return err
		}
		var now_task = task.(Task)
		// fmt.Println(now_task.Id, ":", c.reduce_task_state[now_task.Id])

		c.finished_lock.Lock() 
		if(c.reduce_task_state[now_task.Id] == 0 || c.reduce_task_state[now_task.Id] == 3){
			defer c.finished_lock.Unlock()
			// fmt.Println(now_task.Id, ":", c.reduce_task_state[now_task.Id])
			//不存在 | 做完了
			resp.Woker_Task = Task{
				Type: WaitReduce,
				Id: -1,
				FileName: "",
				StartTime: "",
			}
			err = c.free_maper_task.pop()

			if(err != nil){
				return errors.New("ReduceTask, none or finished")
			}
			return nil
		}
		c.finished_lock.Unlock()

		//ready || running
		resp.Woker_Task = now_task
		resp.Woker_Task.StartTime = time.Now().Format("2006-01-02 15:04:05")
		err = c.free_reducer_task.pop()

		if(err != nil){
			fmt.Println("pop is error")
			return errors.New("ReduceTask: pop fail, len is zero")
		}

		//将任务的值修改为运行中
		c.finished_lock.Lock()
		c.reduce_task_state[now_task.Id] = 2
		c.finished_lock.Unlock()
		
		return nil

	case WaitMap:
		if(c.free_maper_task.len() == 0){
			c.state_lock.Lock()
				c.state = Reduce_Task
			c.state_lock.Unlock()
			resp.Woker_Task = Task{
				Type: WaitMap,
				Id: -1,
				FileName: "",
				StartTime: "",
			}
		}

		//取任务
		task,err := c.free_maper_task.top()
		if(err != nil){
			return err
		}

		var now_task = task.(Task)
		now_task.StartTime =  time.Now().Format("2006-01-02 15:04:05")
		now_task.Type = WaitMap

		//WaitMap
		resp.Woker_Task = now_task
		
		err = c.free_maper_task.pop()

		if(err != nil){
			return errors.New("MapTask: pop fail, len is zero")
			// return err
		}

	case WaitReduce:
		if(c.free_reducer_task.len() == 0){
			c.state_lock.Lock()
				c.state = Success
			c.state_lock.Unlock()
			resp.Woker_Task = Task{
				Type: Success,
				Id: -1,
				FileName: "",
				StartTime: "",
			}
		}

		//取任务
		task,err := c.free_reducer_task.top()
		if(err != nil){
			return err
		}

		var now_task = task.(Task)
		now_task.StartTime =  time.Now().Format("2006-01-02 15:04:05")
		now_task.Type = Success

		//WaitMap
		resp.Woker_Task = now_task
		
		err = c.free_reducer_task.pop()

		if(err != nil){
			return errors.New("MapTask: pop fail, len is zero")
			// return err
		}

	case Success:
		resp.Woker_Task = Task{
			Type: Success,
			Id: -1,
			FileName: "",
			StartTime: "",
		}
	}
	return nil
}

func (c *Coordinator)FinishMapTask(req *MapTaskReq, resp *MapTaskResp)error{
	//Task_Id
	c.finished_lock.Lock()
	defer c.finished_lock.Unlock()
	task_id := req.Task_Id
	if(c.map_task_state[task_id] == 2){
		c.map_task_state[task_id] = 3
		c.map_finish_cnt++
		if(c.map_finish_cnt == c.map_cnt){
			c.ok_lock.Lock()
				c.map_ok = true
			c.ok_lock.Unlock()
			c.initReduceTasks()
			c.state_lock.Lock()
				c.state = WaitMap
			c.state_lock.Unlock()
		}
	}
	return nil
}

func (c *Coordinator)FinishReduceTask(req *ReduceTaskReq, resp *ReduceTaskResp)error{
	c.finished_lock.Lock()
	defer c.finished_lock.Unlock()
	task_id := req.Task_Id
	if(c.reduce_task_state[task_id] == 2){
		// fmt.Printf("reduce %d is ok\n", task_id)
		c.reduce_task_state[task_id] = 3
		c.reduce_finish_cnt++
		if(c.reduce_finish_cnt == c.reduce_cnt){
			c.ok_lock.Lock()
				c.reduce_ok = true
			c.ok_lock.Unlock()
			c.state_lock.Lock()
				c.state = WaitReduce
			c.state_lock.Unlock()
		}
	}
	return nil
}

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}


//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	ret := false

	// Your code here.
	c.ok_lock.Lock()
	ret = c.map_ok && c.reduce_ok
	c.ok_lock.Unlock()

	// now_state := c.getState()
	// fmt.Println("state :", now_state)

	if(ret){
		time.Sleep(2 * time.Second)
	}

	return ret
}


func(c *Coordinator) initWorkerId(){
	i:=0
	for i < c.map_cnt{
		c.free_maper_id.push_back(i)
		i++
	}
	i=0
	for i < c.reduce_cnt{
		c.free_reducer_id.push_back(i)
		i ++
	}
}

func(c *Coordinator) initMapTasks(files []string){
	i := 0
	for i < c.map_cnt{
		task := Task{
			Type: Map_Task,
			Id: i,
			FileName: files[i],
			Reduce_cnt: c.reduce_cnt,
		}
		//将任务放入任务队列，然后任务状态修改为1：ready
		c.free_maper_task.push_back(task)
		c.map_task_state[i] = 1
		// fmt.Println(task.Id, ",", task.FileName)
		i++
	}
	// fmt.Println("map task init over")
}

func(c *Coordinator) initReduceTasks(){
	c.task_lock.Lock()
	defer c.task_lock.Unlock()
	i := 0
	for i < c.reduce_cnt{
		task := Task{
			Type: Reduce_Task,
			Id: i,
			FileName: "",
			Reduce_cnt: c.reduce_cnt,
		}
		//将任务放入任务队列，然后任务状态修改为1：ready
		c.free_reducer_task.push_back(task)
		c.reduce_task_state[i] = 1
		// fmt.Printf("reduce task %d init ok\n", task.Id)
		i++
	}
	fmt.Println("reduce task init over")
}

func timer(c *Coordinator, files []string){
	for{
		now_state := c.getState()
		i := 0
		var task_cnt int   
		c.task_lock.Lock()
		switch now_state {
		case Map_Task:
			task_cnt = c.map_cnt
			for i < task_cnt{
				var flag = false
				c.finished_lock.Lock()
					if(c.map_task_state[i] == 2){
						flag = true;
					}
				c.finished_lock.Unlock()
				if(flag){
					task := Task{
						Type: Map_Task,
						Id: i,
						FileName: files[i],
						Reduce_cnt: c.reduce_cnt,
					}
					c.free_maper_task.push_back(task)
					break
				} else{
					i++
				}
				
			}
		case Reduce_Task:

			task_cnt = c.reduce_cnt
			for i < task_cnt{
				var flag = false
				c.finished_lock.Lock()
					if(c.reduce_task_state[i] == 2){
						flag = true;
					}
				c.finished_lock.Unlock()
				if(flag){
					// goto AddReduceTask
					// c.task_lock.Lock()
					task := Task{
						Type: Reduce_Task,
						Id: i,
						FileName: "",
						Reduce_cnt: c.reduce_cnt,
					}
					c.free_reducer_task.push_back(task)
					// c.task_lock.Unlock()
					break
				} else{
					i++
				}
				// i++
			}
			// AddReduceTask:
				
		case Success:
			// goto Exit
			return
			//退出
		}
		c.task_lock.Unlock()
		time.Sleep(1 * time.Second)
	}
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{
		state: Coor_Start,
		free_maper_id: construct_Queue(),
		free_reducer_id: construct_Queue(),
		map_cnt: len(files),
		reduce_cnt: nReduce,
		free_maper_task: construct_Queue(),
		free_reducer_task: construct_Queue(),
		map_task_state: make(map[int]int),
		reduce_task_state: make(map[int]int),
		map_finish_cnt: 0,
		reduce_finish_cnt: 0,
		map_ok: false,
		reduce_ok: false,
	}

	c.initWorkerId()
	c.initMapTasks(files)
	c.state = Map_Task

	go timer(&c, files)

	c.server()
	return &c
}

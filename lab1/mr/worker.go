package mr

import (
	"fmt"
	"hash/fnv"
	"log"
	"net/rpc"
	"time"
	"os"
	"io/ioutil"
	"strconv"
	"path/filepath"
	"encoding/json"
	"sort"
)

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

type ByKey []KeyValue
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}


//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.

	for{
		// worker_id,task_info := CallWorkerId()
		task_info := CallTask()
		// fmt.Println("WorkerType :", task_info.Type)
		// fmt.Println(task_info.Id, ":", task_info.FileName)
		switch task_info.Type{
		case Map_Task:
			// fmt.Println(task_info.StartTime)
			// fmt.Println(task_info.Id, ":", task_info.FileName)
			doMapWorker(&task_info, mapf)
		case Reduce_Task:
			// fmt.Println("Reduce_Task:")
			// fmt.Println(task_info.StartTime)
			// fmt.Println(task_info.Id)
			doReduceWorker(&task_info, reducef)
		case WaitMap:

		case WaitReduce:
			// CompleteTask()
		case Success:
			return
		}
		time.Sleep(1 * time.Second)
	}

	// uncomment to send the Example RPC to the coordinator.
	// CallExample()

}

func doReduceWorker(task_info *Task, reducef func(string, []string) string){
	//匹配map输出文件	这里由于是本机，直接读。但是实际上在不同机器上文件可能在不同的机器上，这里要传文件
	var pattern string
	pattern = fmt.Sprintf("map-to-reduce%d*", task_info.Id)
	

    files, err := filepath.Glob(pattern)
    if err != nil {
        fmt.Println("无法匹配文件:", err)
        return
    }
	//通过json解析文件
	intermediate := []KeyValue{}
    if len(files) == 0 {
        fmt.Println("没有找到匹配的文件.")
    } else {
        for _, file := range files {
            content, err := ioutil.ReadFile(file)
            if err != nil {
                fmt.Println("无法读取文件:", err)
                continue
            }
			
			//json解析文件
			KV_slice := []KeyValue{}
    		err = json.Unmarshal(content, &KV_slice)
			if(err != nil){
				fmt.Println(err)
				return
			}
			intermediate = append(intermediate, KV_slice...)
            // fmt.Println("文件名:", file)
            // fmt.Println("文件内容:")
            // fmt.Println(string(content))
			os.Remove(file)
        }
    }

	
	sort.Sort(ByKey(intermediate))

	//reduce 操作
	oname := fmt.Sprintf("mr-out-%d", task_info.Id)
	ofile, _ := os.Create(oname)

	i := 0
	for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}
		output := reducef(intermediate[i].Key, values)

		// this is the correct format for each line of Reduce output.
		fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)

		i = j
	}

	ofile.Close()
	
	CallReduceComplete(task_info.Id)
}

//删除最后一个,
func delLastComma(tempio *os.File) error{
	fileInfo, _ := tempio.Stat()
	fileSize := fileInfo.Size()
	if fileSize > 2{
		_, err := tempio.Seek(fileSize-2, 0)
		if err != nil {
			fmt.Println(err)
			return err
		}	
		var lastChar []byte = make([]byte, 1)
    	_, err = tempio.Read(lastChar)
        if err != nil {
        	fmt.Println(err)
            return err
        }
		// 判断最后一个字符是否为 ,
		if string(lastChar) == "," {
		// fmt.Println("最后一个字符是 ,")
		_, err = tempio.Seek(fileSize-2, 0)
        if err != nil {
            return err
        }
		 // 将逗号替换为一个空格
		 _, err = tempio.Write([]byte(" "))
		 if err != nil {
			 return err
		 }
		// fmt.Println("最后一个字符,已替换")
			// 在此处可以执行删除最后一个字符的操作，类似之前所示的方法
		} 
	}
	return nil
}

func GroupMapFile(intermediate []KeyValue, task_info *Task) error{
	// 创建一个临时目录
	// tmpdir, err := ioutil.Dir("", "tempdir-example")
	// if err != nil {
    //     fmt.Println("Error creating temporary directory:", err)
    //     return "",err
    // }
	// defer os.RemoveAll(tmpdir) // 在程序退出时删除临时目录.
	//根据 Reduce_cnt 创建临时文件
	name_pre := "map-to-reduce"	//map-to-reduce-x-Y	x是reduce任务，Y是当前任务
	tempfiles := []string{}						
	tempfileio := []*os.File{}
	for i := 0; i < task_info.Reduce_cnt; i++{
		tempfilename := fmt.Sprintf("%s-%d-%d.txt", name_pre,i,task_info.Id)
		
		file,err := ioutil.TempFile("./", tempfilename)
		
		if err != nil {
			fmt.Printf("Error creating file %s: %v\n", tempfilename, err)
			return err
		}
		tempfiles = append(tempfiles, file.Name())
		tempfileio = append(tempfileio, file)

		file.WriteString("[\n")
	}

	for _,kva := range intermediate{
		index := ihash(kva.Key)
		index %= task_info.Reduce_cnt

		write_content := fmt.Sprintf("{\n\"Key\":\"%s\",\n\"Value\":\"%s\"\n},\n", kva.Key, kva.Value)
		// fmt.Println(write_content)
		tempfileio[index].WriteString(write_content)
	}

	for _,tempio := range tempfileio{
		err := delLastComma(tempio)
		tempio.WriteString("\n]")
		tempio.Close()
		if(err != nil){
			fmt.Println(err)
			return err
		}
	}

	for id,temppath := range tempfiles{
		newFileName := "./map-to-reduce" + strconv.Itoa(id) + "-" + strconv.Itoa(task_info.Id) + ".txt"
		// fmt.Println(newFileName)
		err := os.Rename(temppath, newFileName)
		if err != nil {
    		fmt.Println("修改文件名失败:", err)
    		// return err
		}
	}

	return nil
}

func doMapWorker(task_info *Task,  mapf func(string, string) []KeyValue,){
	intermediate := []KeyValue{}
	file, err := os.Open(task_info.FileName)
	if err != nil {
		log.Fatalf("cannot open %v", task_info.FileName)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", task_info.FileName)
	}
	file.Close()
	kva := mapf(task_info.FileName, string(content))
	intermediate = append(intermediate, kva...)
	err = GroupMapFile(intermediate, task_info)
	if(err != nil){
		fmt.Println(err)
		return
	}
	//任务完成
	CallMapComplete(task_info.Id)
}

/*
*	获取任务
*/
func CallTask()(task_info Task){
	args := TaskReq{}
	reply := TaskResp{}

	call("Coordinator.GetTask", &args, &reply)
	return reply.Woker_Task
}

/*
*	Map任务完成
*/
func CallMapComplete(task_id int){
	args := MapTaskReq{}
	reply := MapTaskResp{}
	args.Task_Id = task_id
	
	call("Coordinator.FinishMapTask", &args, &reply)
}

/*
*	Reduce任务完成
*/
func CallReduceComplete(task_id int){
	args := MapTaskReq{}
	reply := MapTaskResp{}
	args.Task_Id = task_id
	
	call("Coordinator.FinishReduceTask", &args, &reply)
}

/*
*	Worker 没有任务
*/
// func CompleteTask(){
// 	args := CompleteWorkerArgs{}
// 	reply := CompleteWorkerReply{}

// 	call()
// }

//
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	call("Coordinator.Example", &args, &reply)

	// reply.Y should be 100.
	fmt.Printf("reply.Y %v\n", reply.Y)
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}

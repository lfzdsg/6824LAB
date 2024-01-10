对于实验1 mapreduce 的理解
结果是文件中的txt的单词计数，使用pg-*.txt描述
mrworker.go 接受wc.so参数，然后在该运送参数中寻找Map和Reduce函数，传给Worker
    mapf, reducef := loadPlugin(os.Args[1])
    其中mapf是传入filename和content，然后获取content中的单词，返回{word,"1"}
    reducef是传入key和value数组，由于全是1，我们可以知道key出现的次数

mrcoordinator.go 接收pg-*.txt，然后传入coordinator.go得到一个coordinator
    我们知道在mrsequential.go中我们通过参数pg-*.txt得到所有txt后用ReadAll
    得到content。将所有content传入mapf，得到字符串值，排序后传入reducef得到key的出现次数。

    方案1：
    将不同的pg*.txt给不同的worker处理。使用rpc传送文件名，启动一个新的gorouting来做单词处理。
    当某个Map任务完成后会得到一个字符串切片，传到coordinator.go处理。分组和排序。
    Worker启动reduce任务，向coordinator.go请求数据。
    reduce将结果传到coordinator.go,coordinator.go存入disk。

    方案2：
    Map不把结果返回给coor,只返回完成结果。
    Map任务完成后，将结果分组后存入不同临时文件，将临时文件名传给coor。启动reduce任务。
    reduce任务向coor请求临时文件路径，然后读取文件，排序，reducef，得到结果，传给coor。
    coor在收到reduce完成后就将reduce结果存入disk

2023/11/7
    测试发现问题：
    1.map,reduce函数是动态库提供，不同库函数不同，结果不同
    2.会启动多个Worker来请求任务。一直以为是一个Worker启动任务，然后该Worker安排任务，现在看来是启动多个Worker，然后coor安排任务
    3.没有维护任务的状态
    4.我们有nReduce，该变量作用是干什么的，在之前认为是reduce任务的数量，现在看来任务数量只由Worker决定
    5.现在是启动不同的Worker，在某个Worker结束后，临时文件会被销毁，所以这里要么一直运行Worker，要么使用正常文件存储
    
    解决方案：我们的任务安排由coor决定。意思是Worker只管运行任务，返回处理完成结果，不管任务分配。
    在一开始，coor接收到了参数后就初始化任务，如任务Id，类型，名字，是否完成，开始时间。
    Worker只管申请任务，分析任务类型后处理相关任务，然后返回结果，coor处理结果，最后完成退出。
        （1）虽然不同的动态库提供的库函数不同，但是总的处理逻辑是相似的
        （2）如上描述，Worker只管运行任务，任务的调度由coor管理
        （3）可以维护了
        （4）nReduce是Reduce任务的数量，但是该数量不是说我要同时运行nReduce个任务，而是有nReduce个任务需要运行
        （5）我们用临时文件先存储中间结果，然后写完后使用os.rename重命名
    
    现在关于任务重传的问题。
        如果某个Worker在执行Map任务的时候被阻塞了，或者进程被杀死，这个时候我们需要启动任务重传。
        关于这个任务重传机制，该怎么做。
        1.不断的往任务队列中放入任务，如果放完了，那就继续放进行中的任务。
            该做法有效可行，关键在于阻塞的进程创建的临时文件不能很好的删除。我在Map阶段将产生的文件放入创建的临时目录中，然后创建临时文件存入结果，将目录返回到coor存储，然后再发送给reduce。定时器可以设计为两秒加一个进行中的任务进入任务队列。
            (1)怎么处理多出来的临时文件呢？不处理，测试命令后面会删除文件夹，不管，大不了自己删。
        2.当某种类型的任务都完成了，如果直接启动reduce任务，存在某些map任务还在运行，测试发现当reduce需要的文件被map正在访问修改会出问题，所以要让还未运行完成的Map任务完成。
            我添加了两种新状态，WaitMap和WaitReduce，这两个状态我们通过判断任务队列是否为空来修改状态，这可以做一类似等待未完成任务完成的操作，如果任务被杀死了，不影响，如果任务阻塞了，新任务只能是Wait或者Reduce/Success，任务成功了，不影响。
    

go run -race mrsequential.go wc.so pg*.txt
go run -race mrsequential.go indexer.so pg*.txt
go run -race mrsequential.go mtiming.so pg*.txt
go run -race mrsequential.go nocrash.so pg*.txt

go build -race -buildmode=plugin ../mrapps/wc.go
go build -race -buildmode=plugin ../mrapps/indexer.go
go build -race -buildmode=plugin ../mrapps/mtiming.go
go build -race -buildmode=plugin ../mrapps/nocrash.go

rm mr-out*
go run -race mrcoordinator.go pg-*.txt
go run -race mrcoordinator.go lfz-text.txt

go run -race mrworker.go wc.so
go run -race mrworker.go indexer.so
go run -race mrworker.go mtiming.so
go run -race mrworker.go nocrash.so

bash test-mr.sh > debug.txt
bash lfz-test.sh > debug.txt
sort mr-out* | grep . > mr-crash-all



实验二: go test -run 2A -race > debug.txt
go test -run TestReElection2A -race > debug.txt 
go test -run TestManyElections2A -race > debug.txt 
2A: 选主
    [博客](https://coderatwork.cn/posts/notes-on-raft-1/)

    raft有一种心跳机制，如果存在leader，就周期性的向所有的follower发送心跳维持地位。
    如果一段时间没有收到心跳，该节点就认为系统中没有可用的leader，然后开始选举。
    开始一个选举后，f先增加自己的当前**任期号**并转换到c状态，投票给自己，然后并行的向其他节点发送投票请求。
    1. 获得超过半数选票赢得选举 -> 成为leader发送心跳
    2.其他节点赢得了选举 -> 收到新的心跳后，如果新的leader任期号不小于自己当前的任期号，那么从c退回f。
    3.一段时间之后没有任何的获胜者 -> 每个c都在一个自己的随机选举超时时间后增加任期号开始新一轮投票。

        RequestVoteReq:
        {
            dterm           //自己当前的任期号
            candidateId     //自己的Id
            lastLogIndex    //自己最后一个日志号
            lastLogTerm     //自己最后一个日志的任期
        }

        RequestVoteResp:
        {
            term        //自己当前任期号
            voteGranted //是否投票
        }

    本实验中，我们定义数据：
    state int       //当前状态 ：follower,candidate,leader
	term int        //当前任期
	log []logEntry  //日志
	voted bool      //是否投票
    lastHeartbeatTime time.Time    
         
    心跳的理解：任务开始，给每一个节点一个定时器，该定时器结束前如果没有接收到leader的心跳就变成选举，然后开始一轮选举。所以在之后的时间里，我们的选举者的定时器用来重启选举任务。成为leader后该定时器用来分发心跳，保证自己的leader状态。
    问题在于：该定时器是怎么设计定时时间。
        我们可以看到，有两个定时器：
        1.选举超时定时器，该定时器用来控制F，C的选举超时。一般是 [150, 300]ms。对于该定时器我的疑问在于我们在一轮投票结束之前可能还没有接收到所有RPC的结果，这时候没能及时改变状态启动心跳但是定时器到时间了，这时候怎么看。该定时器在什么时候会进行更新。
        2.leader心跳定时器。因为leader需要心跳来维持自己的leader地位，在论文中写道我们要并行的向所有节点发送心跳，我的疑问在于我们发送心跳要比选举超时定时器要短，怎么平衡这个时间。

        

    对于接受选举voteRPC的节点，其状态也是三种：F，C，L。
    F:收到vote请求，先判断请求任期与自己当前任期的区别。
        大于 --> 请求节点是合法的。这时说明当前节点还没有到目标任期，所以没有投过票，直接将票投过去，将任期修改为该任期。
        等于 --> 正常情况下两个节点不会有相同，此时如果相同只可能是当前节点节点也是C。也可能是L，但是这说明之前当前此时的请求节点存在错误的运行情况，也就是错过了某些任期，所以请求节点不应该当选。综上该情况不投票。
        小于 --> 请求节点不合法。不投票。

    C:发送vote请求。收到投票的信息。
        投 --> 将总票数加1
        不投 --> 不管。如果发现返回的任期也比自己大，这时候说明当前机器可能在之前有过崩溃，没有参与某些任期，这会使得当前机器选举失败。所以不用管。

    L:处理与F类似，并且不用修改当前的状态。我们可以认为状态在之后的心跳过程中修改
    
    leader开始发送心跳到每个节点，大致过程是节点接收到了请求后判断任期和日志请求，2A还没有详细实现，目前我只管选主，不管日志。所以目前是只判断任期，如果任期更大就改变状态成F。


2B:日志复制
    go test -run TestBasicAgree2B > debug.txt
    go test -run TestFailAgree2B > debug.txt
    go test -run TestRejoin2B > debug.txt
    go test -run TestBackup2B > debug.txt
    
    go test -run 2B > debug.txt

    日志包含三个信息，状态机指令，leader的任期号，日志号。
    1-通常我们认为如果两个日志记录有相同的索引和任期，那么这两条日志记录中的命令也相同
    2-如果两个日志的两条日志记录有相同的索引和任期，那么这两个日志中的前继日志记录也是相同的。

    leader并行发送AppendEntries RPC给Follower,让他们复制该条目。当该条目被超过半数的follower复制后，leader就可以在本地执行该指令并把结果返回客户端。

    我们把本地执行指令，也就是leader应用日志与状态机这一步称为提交。

    RAFT的一致性检查:leader在每一个发送follower的追加条目RPC中，会放入前一个日志条目的索引位置和任期号，如果follower在它的日志找不到前一个日志，那么它就会拒绝此日志，leader收到follower的拒绝后，会发送前一个日志条目，从而逐渐向前定位到follower第一个缺失的的日志。

    三种情况:
        1.follower因为某些原因没有给leader响应，那么leader会不断重发追加条目请求，哪怕leader已经回复了客户端。
        2.如果有follower崩溃后恢复，这时RAFT追加条目的一致性检查生效，保证follower能按顺序恢复崩溃后的缺失的日志。
        3.leader崩溃，那么崩溃的leader可能已经复制了日志到部分follower但还没有提交，而被选出的新leader有可能不具备这些日志，这样就有部分follower中的日志和新leader的日志不相同。--RAFT在这这种情况下强制follower复制自己的日志解决不一致，这意味着follower中跟leader冲突的日志条目会被新Leader的日志条目覆盖（因为没有提交所以不违背外部一致性）。

    实现上：
        现在我们希望实现日志复制，讲讲具体实现。client那边发送命令到leader，这时会包含一个command，我们将这个command存入日志文件中。
        然后再在一次选主结束后，我们就发送心跳把日志发送到所有的节点。
        我们维护几个东西。
        所有机器维护一个commitIndex，和lastApplied。这两个分别的作用：下一个提交的日志索引，已经提交的最后一个日志索引。

        commitIndex是下一个要提交的日志索引，我们知道提交就是说一个命令被超过半数的机器复制过后在状态机运行就算提交，疑问是对于follower节点来说它怎么知道该条目是提交的。那么lastApplied的作用就是这个，我们可以认为如果一个新的日志复制过来了，那么前面的日志就是提交的。
        不过我们会在RPC中发送leader的commitIndex，所以和leader同步就好。
        
        leader维护nextIndex，matchIndex。
        其中nextIndex是下一个发送的日志，matchIndex是下一个要复制的索引。
        
        
        我们简单模拟一下，在一次选举结束后，现在有一个leader节点和一群follower节点。他们的日志都是初始化的空日志，我们会在开始的时候初始化一个日志，这个日志只有一条信息，那就是{index:0,lterm:0,command:nil}。现在leader接收到一个命令,将命令放到自己的日志中，这时候开启一轮心跳的时候就把该命令发到其他机器。然后我们根据和返回的结果更新nextIndex和matchIndex。而且当matchIndex中超过半数的值都超过了某个值说明commitIndex应该自增。
        我们发送
            term leader 任期
            leaderId 用来 follower 重定向到 leader
            prevLogIndex 前继日志记录的索引
            prevLogItem 前继日志的任期
            entries[] 存储日志记录
            leaderCommit leader 的 commitIndex

        //2B
	    log 				[]LogEntry	//日志记录每条日记记录包含状态机的命令和从 leader 接受到日志的任期。(索引初始化为 1)
	    logLock				sync.Mutex
	    //所有机器的可变状态
	    commitIndex 		int		//将被提交的日志记录的索引（初值为 0 且单调递增）
	    lastApplied			int	 	//已经被提交到状态机的最后一个日志的索引（初值为0 且单调递增）
	    //leader 的可变状态：
	    nextIndex		[]int 	//每台机器在数组占据一个元素，元素的值为下条发送到该机器的日志索引 (初始值为 leader 最新一条日志的索引 +1)
	    matchIndex			[]int 	//每台机器在数组中占据一个元素，元素的记录将要复制给该机器日志的索引。

        log是命令日志，我想这个是Start那里传过来的日志命令。这个命令我们追加到leader的日志中。
        commitIndex 这里指的是将要提交的日志位置，也就是心跳将要发送的指令
	    lastApplied 指的是上一个提交的指令索引
        nextIndex   这里维护每一个节点
        


问题：
    config.go:550: one(106) failed to reach agreement
    
    panic: runtime error: index out of range [6] with length 2
    这里说的是当前节点的日志只有2个，但是leader为当前节点维持的前置日志是6，所以有问题，关键是为什么当前节点没有把日志复制完全。
    找到了，问题是因为传过来的前置日志比现在节点拥有的日志要大，还要判断长度

     FAIL: TestBackup2B (24.51s)
    config.go:550: one(2165423027531050681) failed to reach agreement
    错误原因：之前是因为在网络分区后重新苏醒的三个节点没有形成新的网络，现在是我们的任务提交太慢导致当前的指令没有完成提交。大概的原因是新的leader的前继日志错误然后一直往前找到第一个没有匹配的日志，这需要很多次RPC，理论上是可行的，但是当前实验是有时间限制的。
    我们知道论文中对这里描述的是如果复制的日志不对那么nextindex--,但是现在的话我们要把这个过程减少到几次就好。
    -二分：二分应该怎么写
    
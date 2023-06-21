package internal

// 推送任务
type PushJob struct {
	pushType int         // 推送类型
	roomId   string      // 房间ID
	bizMsg   *BizMessage // 未序列化的业务消息
	wsMsg    *WSMessage  //  已序列化的业务消息
}

// 连接管理器
type ConnManager struct {
	buckets []*Bucket
	jobChan []chan *PushJob // 每个Bucket对应一个Job Queue

	dispatchChan chan *PushJob // 待分发消息队列
}

var (
	G_connMgr *ConnManager
)

// 消息广播到所有Bucket
func (connMgr *ConnManager) dispatchWorkerMain(dispatchWorkerIdx int) {
	var (
		bucketIdx int
		err       error
	)

	for pushJob := range connMgr.dispatchChan {
		DispatchPending_DESC()
		if pushJob.wsMsg, err = EncodeWSMessage(pushJob.bizMsg); err != nil {
			continue
		}

		// 分发给所有Bucket, 若Bucket拥塞则等待
		for bucketIdx, _ = range connMgr.buckets {
			PushJobPending_INCR()
			connMgr.jobChan[bucketIdx] <- pushJob
		}
	}
}

// Job负责消息广播给客户端
func (connMgr *ConnManager) jobWorkerMain(jobWorkerIdx int, bucketIdx int) {
	var (
		bucket  = connMgr.buckets[bucketIdx]
		pushJob *PushJob
	)

	for {
		select {
		case pushJob = <-connMgr.jobChan[bucketIdx]: // 从Bucket的job queue取出一个任务
			PushJobPending_DESC()
			if pushJob.pushType == PUSH_TYPE_ALL {
				bucket.PushAll(pushJob.wsMsg)
			} else if pushJob.pushType == PUSH_TYPE_ROOM {
				bucket.PushRoom(pushJob.roomId, pushJob.wsMsg)
			}
		}
	}
}

func InitConnManager() (err error) {
	var (
		bucketIdx         int
		jobWorkerIdx      int
		dispatchWorkerIdx int
		connMgr           *ConnManager
	)

	connMgr = &ConnManager{
		buckets:      make([]*Bucket, GlobalConfig.BucketCount),
		jobChan:      make([]chan *PushJob, GlobalConfig.BucketCount),
		dispatchChan: make(chan *PushJob, GlobalConfig.DispatchChannelSize),
	}
	for bucketIdx, _ = range connMgr.buckets {
		connMgr.buckets[bucketIdx] = NewBucket(bucketIdx)                                   // 初始化Bucket
		connMgr.jobChan[bucketIdx] = make(chan *PushJob, GlobalConfig.BucketJobChannelSize) // Bucket的Job队列
		// Bucket的Job worker
		for jobWorkerIdx = 0; jobWorkerIdx < GlobalConfig.BucketJobWorkerCount; jobWorkerIdx++ {
			go connMgr.jobWorkerMain(jobWorkerIdx, bucketIdx)
		}
	}
	// 初始化分发协程, 用于将消息扇出给各个Bucket
	for dispatchWorkerIdx = 0; dispatchWorkerIdx < GlobalConfig.DispatchWorkerCount; dispatchWorkerIdx++ {
		go connMgr.dispatchWorkerMain(dispatchWorkerIdx)
	}

	G_connMgr = connMgr
	return
}

func (connMgr *ConnManager) GetBucket(wsConnection *WSConnection) (bucket *Bucket) {
	bucket = connMgr.buckets[wsConnection.connId%uint64(len(connMgr.buckets))]
	return
}

func (connMgr *ConnManager) AddConn(wsConnection *WSConnection) {
	connMgr.GetBucket(wsConnection).AddConn(wsConnection)
	OnlineConnections_INCR()
}

func (connMgr *ConnManager) DelConn(wsConnection *WSConnection) {
	connMgr.GetBucket(wsConnection).DelConn(wsConnection)
	OnlineConnections_DESC()
}

func (connMgr *ConnManager) JoinRoom(roomId string, wsConn *WSConnection) (err error) {
	err = connMgr.GetBucket(wsConn).JoinRoom(roomId, wsConn)
	return
}

func (connMgr *ConnManager) LeaveRoom(roomId string, wsConn *WSConnection) (err error) {
	err = connMgr.GetBucket(wsConn).LeaveRoom(roomId, wsConn)
	return
}

// 向所有在线用户发送消息
func (connMgr *ConnManager) PushAll(bizMsg *BizMessage) (err error) {
	var (
		pushJob *PushJob
	)

	pushJob = &PushJob{
		pushType: PUSH_TYPE_ALL,
		bizMsg:   bizMsg,
	}

	select {
	case connMgr.dispatchChan <- pushJob:
		DispatchPending_INCR()
	default:
		err = ERR_DISPATCH_CHANNEL_FULL
		DispatchFail_INCR()
	}
	return
}

// 向指定房间发送消息
func (connMgr *ConnManager) PushRoom(roomId string, bizMsg *BizMessage) (err error) {
	var (
		pushJob *PushJob
	)

	pushJob = &PushJob{
		pushType: PUSH_TYPE_ROOM,
		bizMsg:   bizMsg,
		roomId:   roomId,
	}

	select {
	case connMgr.dispatchChan <- pushJob:
		DispatchPending_INCR()
	default:
		err = ERR_DISPATCH_CHANNEL_FULL
		DispatchFail_INCR()
	}
	return
}

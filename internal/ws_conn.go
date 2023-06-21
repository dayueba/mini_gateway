package internal

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

// 也是一个client
type WSConnection struct {
	mutex             sync.RWMutex
	connId            uint64
	conn              *websocket.Conn
	readChan          chan *WSMessage
	writeChan         chan *WSMessage
	closeChan         chan struct{}
	isClosed          bool
	lastHeartbeatTime time.Time // 最近一次心跳时间
	heartbeat         int       // 心跳，单位秒

	conf *Config

	// user info
	UserId   int
	RoomId   string
	RoomType string
	rooms    map[string]bool // 加入了哪些房间
}

func (conn *WSConnection) readLoop() {
	var (
		msgType int
		msgData []byte
		message *WSMessage
		err     error
	)
	for {
		if msgType, msgData, err = conn.conn.ReadMessage(); err != nil {
			goto ERR
		}

		message = NewWSMessage(msgType, msgData)

		select {
		case conn.readChan <- message:
		case <-conn.closeChan:
			goto CLOSED
		}
	}

ERR:
	conn.Close()
CLOSED:
}

func (conn *WSConnection) writeLoop() {
	var (
		message *WSMessage
		err     error
	)
	for {
		select {
		case message = <-conn.writeChan:
			if err = conn.conn.WriteMessage(message.MsgType, message.MsgData); err != nil {
				goto ERR
			}
		case <-conn.closeChan:
			goto CLOSED
		}
	}
ERR:
	conn.Close()
CLOSED:
}

func NewWSConnection(connId uint64, conn *websocket.Conn, conf *Config) (wsConnection *WSConnection) {
	wsConnection = &WSConnection{
		conn:              conn,
		connId:            connId,
		readChan:          make(chan *WSMessage, 1000),
		writeChan:         make(chan *WSMessage, 1000),
		closeChan:         make(chan struct{}),
		lastHeartbeatTime: time.Now(),
		rooms:             make(map[string]bool),
		heartbeat:         conf.Heartbeat,
		conf:              conf,
	}

	go wsConnection.readLoop()
	go wsConnection.writeLoop()

	return
}

func (conn *WSConnection) SendMessage(message *WSMessage) (err error) {
	select {
	case conn.writeChan <- message:
		SendMessageTotal_INCR()
	case <-conn.closeChan:
		err = ERR_CONNECTION_LOSS
	default: // 写操作不会阻塞, 因为channel已经预留给websocket一定的缓冲空间，目前写满了直接丢弃，当然也可以设置为不丢弃
		err = ERR_SEND_MESSAGE_FULL
		SendMessageFail_INCR()
	}
	return
}

func (conn *WSConnection) ReadMessage() (message *WSMessage, err error) {
	select {
	case message = <-conn.readChan:
	case <-conn.closeChan:
		err = ERR_CONNECTION_LOSS
	}
	return
}

func (conn *WSConnection) Close() {
	if conn.isClosed {
		return
	}

	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	if conn.isClosed {
		return
	}
	conn.conn.Close()
	conn.isClosed = true
	close(conn.closeChan)
}

func (conn *WSConnection) IsAlive() bool {
	var (
		now = time.Now()
	)

	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	// 连接已关闭 或者 太久没有心跳
	if conn.isClosed || now.Sub(conn.lastHeartbeatTime) > time.Duration(conn.heartbeat)*time.Second {
		return false
	}
	return true
}

func (conn *WSConnection) KeepAlive() {
	var (
		now = time.Now()
	)

	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	conn.lastHeartbeatTime = now
}

// 每隔1秒, 检查一次连接是否健康
func (conn *WSConnection) heartbeatChecker() {
	var (
		timer *time.Timer
	)
	timer = time.NewTimer(time.Duration(conn.heartbeat) * time.Second)
	for {
		select {
		case <-timer.C:
			if !conn.IsAlive() {
				conn.Close()
				goto EXIT
			}
			timer.Reset(time.Duration(conn.heartbeat) * time.Second)
		case <-conn.closeChan:
			timer.Stop()
			goto EXIT
		}
	}

EXIT:
	// 确保连接被关闭
}

// 处理PING请求
func (conn *WSConnection) handlePing(bizReq *BizMessage) (bizResp *BizMessage, err error) {
	var (
		buf []byte
	)

	conn.KeepAlive()

	if buf, err = json.Marshal(BizPongData{}); err != nil {
		return
	}
	bizResp = &BizMessage{
		Type: "PONG",
		Data: json.RawMessage(buf),
	}
	return
}

// 处理JOIN请求
func (conn *WSConnection) handleJoin(bizReq *BizMessage) (bizResp *BizMessage, err error) {
	var (
		bizJoinData *BizJoinData
		existed     bool
	)
	bizJoinData = &BizJoinData{}
	if err = json.Unmarshal(bizReq.Data, bizJoinData); err != nil {
		return
	}
	if len(bizJoinData.RoomID) == 0 {
		err = ERR_ROOM_ID_INVALID
		return
	}
	if len(conn.rooms) >= GlobalConfig.MaxJoinRoom {
		// 超过了房间数量限制, 忽略这个请求
		return
	}
	// 已加入过
	if _, existed = conn.rooms[bizJoinData.RoomID]; existed {
		// 忽略掉这个请求
		return
	}
	// 建立房间 -> 连接的关系
	if err = G_connMgr.JoinRoom(bizJoinData.RoomID, conn); err != nil {
		return
	}
	// 建立连接 -> 房间的关系
	conn.rooms[bizJoinData.RoomID] = true
	return
}

// 处理LEAVE请求
func (conn *WSConnection) handleLeave(bizReq *BizMessage) (bizResp *BizMessage, err error) {
	var (
		bizLeaveData *BizLeaveData
		existed      bool
	)
	bizLeaveData = &BizLeaveData{}
	if err = json.Unmarshal(bizReq.Data, bizLeaveData); err != nil {
		return
	}
	if len(bizLeaveData.RoomID) == 0 {
		err = ERR_ROOM_ID_INVALID
		return
	}
	// 未加入过
	if _, existed = conn.rooms[bizLeaveData.RoomID]; !existed {
		// 忽略掉这个请求
		return
	}
	// 删除房间 -> 连接的关系
	if err = G_connMgr.LeaveRoom(bizLeaveData.RoomID, conn); err != nil {
		return
	}
	// 删除连接 -> 房间的关系
	delete(conn.rooms, bizLeaveData.RoomID)
	return
}

func (conn *WSConnection) leaveAll() {
	var (
		roomId string
	)
	// 从所有房间中退出
	for roomId, _ = range conn.rooms {
		G_connMgr.LeaveRoom(roomId, conn)
		delete(conn.rooms, roomId)
	}
}

func (conn *WSConnection) WSHandle() {
	var (
		message *WSMessage
		bizReq  *BizMessage
		bizResp *BizMessage
		err     error
		buf     []byte
	)

	// 连接加入管理器, 可以推送端查找到
	G_connMgr.AddConn(conn)

	// 心跳检测线程
	go conn.heartbeatChecker()

	// 请求处理协程
	for {
		if message, err = conn.ReadMessage(); err != nil {
			goto ERR
		}

		// 只处理文本消息
		if message.MsgType != websocket.TextMessage {
			continue
		}

		// 解析消息体
		if bizReq, err = DecodeBizMessage(message.MsgData); err != nil {
			goto ERR
		}

		bizResp = nil

		// 1,收到PING则响应PONG: {"type": "PING"}, {"type": "PONG"}
		// 2,收到JOIN则加入ROOM: {"type": "JOIN", "data": {"room": "chrome-plugin"}}
		// 3,收到LEAVE则离开ROOM: {"type": "LEAVE", "data": {"room": "chrome-plugin"}}

		// 请求串行处理
		switch bizReq.Type {
		case "PING":
			if bizResp, err = conn.handlePing(bizReq); err != nil {
				goto ERR
			}
		case "JOIN":
			if bizResp, err = conn.handleJoin(bizReq); err != nil {
				goto ERR
			}
		case "LEAVE":
			if bizResp, err = conn.handleLeave(bizReq); err != nil {
				goto ERR
			}
		}

		if bizResp != nil {
			if buf, err = json.Marshal(*bizResp); err != nil {
				goto ERR
			}
			// socket缓冲区写满不是致命错误
			if err = conn.SendMessage(&WSMessage{websocket.TextMessage, buf}); err != nil {
				if err != ERR_SEND_MESSAGE_FULL {
					goto ERR
				} else {
					err = nil
				}
			}
		}
	}

ERR:
	// 确保连接关闭
	conn.Close()

	// 离开所有房间
	conn.leaveAll()

	// 从连接池中移除
	G_connMgr.DelConn(conn)
	return
}

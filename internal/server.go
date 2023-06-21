package internal

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync/atomic"
)

var (
	wsUpgrader = websocket.Upgrader{
		// 允许所有CORS跨域请求
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	curConnId uint64
)

func InitServer(c *Config) (err error) {
	r := gin.Default()
	// ws
	r.GET("/connect", func(ctx *gin.Context) {
		w, r := ctx.Writer, ctx.Request

		// token := r.Header.Get("Authorization")
		// check token & get user_id

		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Fatalln(err)
		}

		// 连接唯一标识
		connId := atomic.AddUint64(&curConnId, 1)

		// 初始化WebSocket的读写协程
		wsConn := NewWSConnection(connId, conn, c)

		// 开始处理websocket消息
		wsConn.WSHandle()
	})

	// http
	r.POST("/push/all", func(c *gin.Context) {
		var msgArr []json.RawMessage
		items := c.PostForm("items")

		if err := json.Unmarshal([]byte(items), &msgArr); err != nil {
			c.AbortWithStatusJSON(500, err)
		}

		for msgIdx, _ := range msgArr {
			G_merger.PushAll(&msgArr[msgIdx])
		}
	})
	r.POST("/push/room", func(c *gin.Context) {
		var msgArr []json.RawMessage
		roomId := c.PostForm("room_id")
		items := c.PostForm("items")

		if err := json.Unmarshal([]byte(items), &msgArr); err != nil {
			c.AbortWithStatusJSON(500, err)
		}

		for msgIdx, _ := range msgArr {
			G_merger.PushRoom(roomId, &msgArr[msgIdx])
		}
	})
	r.GET("/stats", func(ctx *gin.Context) {
		ctx.JSON(200, G_stats)
	})

	// 提供Http接口，返回⻓链接connector节点的IP列表
	r.GET("/v2/live-metadata", func(ctx *gin.Context) {
		// 随机返回4个IP地址
		// 根据客户端信息返回对应的IP协议类型
		// realTcpAddrs = allTcpAddrs
		// if len(allTcpAddrs) > 4 {
		//	realTcpAddrs, err = util.Random(allTcpAddrs, 4) if err != nil {
		//		realTcpAddrs = []string{allTcpAddrs[rand.Intn(len(allTcpAddrs))]}
		//	} }
	})

	return r.Run(fmt.Sprintf(":%d", c.Port))
}

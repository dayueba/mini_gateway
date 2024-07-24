mini gateway
======

定位：**长链接层。（所以可能叫gateway有点不合适，但是自己也懒得改了。）**

常见的设计，一般在长链接层会解析OpCode, 然后再决定发往哪个logic服务，
有哪些OpCode一般是写死在长链接层的，这就意味着如果要扩展的话，需要重启长链接层服务，
大量长链接需要重连，此时我们可以加入一层，可以叫state层，长链接层读取消息后直接转发给state层，
state层解析，再转发给logic。扩展的话 只需要重启state层即可。

1. 多协议客户端接⼊
2. 消息下发及ack确认
3. 连接⼼跳管理
4. 注册客户端信息到redis/内存中
5. 从redis拉取离线消息
6. ⽤户⻓连接管理
7. 未确认消息延迟重发

难点/挑战
- 百万长链接接入
- 每秒70w+消息
- 每天100亿条消息
- 低延迟，高并发，高可靠，高可用

design
1. 接入协议
- websocket，兼容性很好，h5 能用
- quic 弱网优化


1. bucket 是什么？

goim, server 下面挂bucket，conn保存在bucket中。

如果结构是
```go
type server struct {
	conns map[coonID]*net.Conn
	mu sync.Mutex
}
```
如果需要断开连接，此时就需要

2. room 和 bucket 的对应关系

本仓库的方案是，server => buckets => rooms，所以意味着不同buckets下面有相同的room，只是每个room里的conn不同

缺点
1. 一个 room 会存储在多个bucket中，需要遍历所有bucket
2. 一个 user_id 只能支持一个连接

后续优化
1. message 对象池化

网络I/O模型的选择


多个goroutine调用acceptTCP 会不会有问题？

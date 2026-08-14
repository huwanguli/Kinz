# Kinz 线协议（Wire Protocol）

## 默认协议：TLV

Kinz 默认编解码器是 `knet.TLVPack`（实现 `kiface.ICodec`），固定 8 字节头 + 变长载荷：

```
[DataLen: 4 字节][MsgID: 4 字节][Data: DataLen 字节]
```

- **字节序**：默认**小端（little-endian）**；可用 `knet.NewTLVPackWithOrder(binary.BigEndian, max)` 切换大端（自定义 ICodec 同理）。
- **DataLen**：载荷字节数（不含 8 字节头）。
- **MsgID**：路由标识（业务自定义）。
- **上限**：`Config.MaxPacketSize`（默认 4096）；超限的解码返回 `ErrTooLargePacket` 并关闭连接（fail-fast 安全姿态）。

### 示例报文

发送 `MsgID=1, Data="hi"`（小端）：

```
DataLen=2          →  02 00 00 00
MsgID=1            →  01 00 00 00
Data="hi"          →  68 69
整包（10 字节）      →  02 00 00 00 01 00 00 00 68 69
```

大端同样报文：

```
02 00 00 00 01 00 00 00 → 00 00 00 02 00 00 00 01 68 69
```

## 粘包 / 半包

TCP 是字节流，无消息边界。`ICodec` 内部缓冲负责帧重组：

- **粘包**：一次 Read 可能包含多个完整包 → `Decode` 返回多条消息。
- **半包**：一个包分多次到达 → `Decode` 返回 `(nil, nil)`，剩余字节留在内部缓冲，下次继续。
- 框架侧**不需要业务处理粘包**；自定义协议只需实现 `ICodec.Decode/Clone`。

## 保留 MsgID

| MsgID | 用途 |
|-------|------|
| `99999`（`kiface.HeartBeatDefaultMsgID`） | 心跳帧（服务端 `StartHeartBeat` 后周期性发送；存活判定基于任何收到消息，不只心跳帧） |
| `0xFFFFFFFE`（`kiface.ServerFullMsgID`） | 满连接拒绝：`MaxConn` 超限时服务端发送的错误消息（载荷为原因文本），随后关闭连接 |

## 自定义协议

实现 `kiface.ICodec`（分帧 + 解析 + 封包 + Clone 一体），通过 `srv.SetCodec(codec)` 注入：

```go
type ICodec interface {
    Decode(buff []byte) ([]kiface.IMessage, error) // 帧重组 + 消息解析
    Pack(msg kiface.IMessage) ([]byte, error)        // 封包
    Clone() ICodec                                    // 每连接独立实例
}
```

`Decode` 返回的消息 payload 必须独立于 codec 内部缓冲（消息会被异步处理）。

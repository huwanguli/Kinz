package znet

type Message struct {
	Id      uint32 // 消息 ID
	DataLen uint32 // 消息长度
	Data    []byte // 消息内容
}

func NewMessage(id uint32, data []byte) *Message {
	return &Message{
		Id:      id,
		DataLen: uint32(len(data)),
		Data:    data,
	}
}

func (m *Message) GetMsgId() uint32 {
	return m.Id
} // 获取消息 ID

func (m *Message) GetDataLen() uint32 {
	return m.DataLen
} // 获取消息长度

func (m *Message) GetData() []byte {
	return m.Data
} // 获取消息内容

func (m *Message) SetMsgId(id uint32) {
	m.Id = id
} // 设置消息 ID

func (m *Message) SetData(data []byte) {
	m.Data = data
} // 设置消息内容

func (m *Message) SetDataLen(len uint32) {
	m.DataLen = len
} // 设置消息长度

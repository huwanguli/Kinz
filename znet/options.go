package znet

import "zinx/ziface"

type ClientOption func(c ziface.IClient)

func WithPacketClient(pack ziface.IDataPack) ClientOption {
	return func(c ziface.IClient) {
		c.SetPacket(pack)
	}
}

func WithNameClient(name string) ClientOption {
	return func(c ziface.IClient) {
		c.SetName(name)
	}
}

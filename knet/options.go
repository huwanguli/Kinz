package knet

import "kinz/kiface"

type ClientOption func(c kiface.IClient)

func WithPacketClient(pack kiface.IDataPack) ClientOption {
	return func(c kiface.IClient) {
		c.SetPacket(pack)
	}
}

func WithNameClient(name string) ClientOption {
	return func(c kiface.IClient) {
		c.SetName(name)
	}
}

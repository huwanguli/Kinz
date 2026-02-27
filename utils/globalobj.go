package utils

import (
	"encoding/json"
	"io/ioutil"
	"zinx/ziface"
)

// 存储一切有关Zinx的全局参数，供其他模块使用
// 一些参数也可以使用zinx.json由用户进行配置

type GlobalObj struct {
	TcpServer ziface.IServer // 当前Zinx全局的Server对象
	Host      string         `json:"Host"`    // 当前主机监听的IP
	TcpPort   int            `json:"TcpPort"` // 当前主机监听的端口号
	Name      string         `json:"Name"`    // 当前服务器的名称

	// Zinx
	Version          string `json:"Version"`        // 版本
	MaxConn          int    `json:"MaxConn"`        // 允许的最大连接数
	MaxPackageSize   uint32 `json:"MaxPackageSize"` // 允许的数据包最大值
	WorkerPoolSize   uint32 `json:"WorkerPoolSize"` // 当前业务工作池的goroutine的数量
	MaxWorkerTaskLen uint32 // 允许开辟的最大工作池数量
}

// 定义全局的对外GlobalObj

var GlobalObject *GlobalObj

// Reload 冲json中加载
func (g *GlobalObj) Reload() {
	data, err := ioutil.ReadFile("conf/zinx.json")
	if err != nil {
		panic(err)
	}
	// 将json文件数据绑定到对象中
	err = json.Unmarshal(data, g)
	if err != nil {
		panic(err)
	}
}

// init方法,初始化当前的对象

func init() {
	// 如果配置文件没有加载的默认值
	GlobalObject = &GlobalObj{
		Name:             "ZinxServerApp",
		Version:          "v0.4",
		TcpPort:          8999,
		Host:             "0.0.0.0",
		MaxConn:          1024,
		MaxPackageSize:   4096,
		WorkerPoolSize:   10,
		MaxWorkerTaskLen: 1024,
	}

	// 尝试从配置文件中读取
	GlobalObject.Reload()
}

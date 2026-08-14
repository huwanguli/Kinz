package kinterceptor

import (
	"kinz/kiface"
)

type Chain struct {
	req          ziface.IcReq
	position     int
	interceptors []ziface.IInterceptor
}

func (c *Chain) Request() ziface.IcReq {
	return c.req
}

func (c *Chain) GetIMessage() ziface.IMessage {
	req := c.Request()
	if req == nil {
		return nil
	}

	iRequest := c.ShouldIRequest(req)
	if iRequest == nil {
		return nil
	}
	return iRequest.GetMessage()
}

func (c *Chain) Proceed(req ziface.IcReq) ziface.IcResp {
	if c.position < len(c.interceptors) {
		chain := NewChain(c.interceptors, c.position+1, req)
		interceptor := c.interceptors[c.position]
		response := interceptor.Intercept(chain)
		return response
	}
	return req
}

func (c *Chain) ProceedWithIMessage(iMessage ziface.IMessage, response ziface.IcReq) ziface.IcResp {
	if iMessage == nil || response == nil {
		return c.Proceed(c.Request())
	}
	req := c.Request()

	if req == nil {
		return c.Proceed(c.Request())
	}

	iRequest := c.ShouldIRequest(req)
	if iRequest == nil {
		return c.Proceed(c.Request())
	}

	iRequest.SetResponse(response)
	return c.Proceed(iRequest)
}

func NewChain(list []ziface.IInterceptor, pos int, req ziface.IcReq) ziface.IChain {
	return &Chain{
		req:          req,
		position:     pos,
		interceptors: list,
	}
}

func (c *Chain) ShouldIRequest(icReq ziface.IcReq) ziface.IRequest {
	if icReq == nil {
		return nil
	}

	switch icReq.(type) {
	case ziface.IRequest:
		return icReq.(ziface.IRequest)
	default:
		return nil
	}
}

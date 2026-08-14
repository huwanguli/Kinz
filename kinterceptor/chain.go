package kinterceptor

import (
	"kinz/kiface"
)

type Chain struct {
	req          kiface.IcReq
	position     int
	interceptors []kiface.IInterceptor
}

func (c *Chain) Request() kiface.IcReq {
	return c.req
}

func (c *Chain) GetIMessage() kiface.IMessage {
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

func (c *Chain) Proceed(req kiface.IcReq) kiface.IcResp {
	if c.position < len(c.interceptors) {
		chain := NewChain(c.interceptors, c.position+1, req)
		interceptor := c.interceptors[c.position]
		response := interceptor.Intercept(chain)
		return response
	}
	return req
}

func (c *Chain) ProceedWithIMessage(iMessage kiface.IMessage, response kiface.IcReq) kiface.IcResp {
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

func NewChain(list []kiface.IInterceptor, pos int, req kiface.IcReq) kiface.IChain {
	return &Chain{
		req:          req,
		position:     pos,
		interceptors: list,
	}
}

func (c *Chain) ShouldIRequest(icReq kiface.IcReq) kiface.IRequest {
	if icReq == nil {
		return nil
	}

	switch icReq.(type) {
	case kiface.IRequest:
		return icReq.(kiface.IRequest)
	default:
		return nil
	}
}

package geerpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"geerpc/codec"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

//一次RPC调用所需要的信息
type Call struct {
	Seq           uint64      //请求序号
	ServiceMethod string      //服务名.方法名
	Args          interface{} //参数
	Reply         interface{} //返回值
	Error         error       //错误信息
	Done          chan *Call  //用于异步调用
}

//当调用结束之后会调用call.done()通知调用方（异步调用）
func (call *Call) done() {
	call.Done <- call
}

//一个RPC客户端，某一时刻一个Client可能有多个未完成的Call，一个Client也可能同时被多个goroutine调用
type Client struct {
	cc       codec.Codec      //编解码器，用来序列化和反序列化请求
	opt      *Option          //客户端配置
	sending  sync.Mutex       //发送互斥锁，保证发送的请求是有序的，防止多个请求报文混淆
	header   codec.Header     //请求头，只在发送时使用
	mu       sync.Mutex       //保证请求的唯一性
	seq      uint64           //请求序号
	pending  map[uint64]*Call //存储未处理完的请求
	closing  bool             //用户主动关闭
	shutdown bool             //服务端关闭，一般是有错误发生
}

type clientResult struct {
	client *Client
	err    error
}

type newClientFunc func(conn net.Conn, opt *Option) (client *Client, err error)

//实现了io.Closer接口，用于关闭连接
var _ io.Closer = (*Client)(nil)

//错误：连接已经关闭
var ErrShutdown = errors.New("connection is shut down")

// Close the connection
func (client *Client) Close() error {
	//保证关闭连接的原子性
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrShutdown
	}
	client.closing = true
	//关闭io.ReadWriteCloser
	return client.cc.Close()
}

// IsAvailable return true if the client does work
func (client *Client) IsAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.shutdown && !client.closing
}

//注册请求，将请求放入map中
func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing || client.shutdown {
		return 0, ErrShutdown
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil
}

//处理请求
func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()
	//从map中删除请求
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

//服务端或客户端发生错误时调用，将错误信息通知所有未处理完的请求
func (client *Client) terminateCalls(err error) {
	//保证关闭连接的原子性
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()
	//设置错误信息
	client.shutdown = true
	//遍历所有未处理完的请求，通知错误
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

//接收响应
func (client *Client) receive() {
	//接收响应
	var err error
	//循环读取响应
	for err == nil {
		var h codec.Header
		//读取响应头
		if err = client.cc.ReadHeader(&h); err != nil {
			break
		}
		//根据请求序号找到对应的请求
		call := client.removeCall(h.Seq)
		switch {
		//请求不存在，即请求已经被删除
		case call == nil:
			err = client.cc.ReadBody(nil)
		//服务端处理出错，即服务端返回的Header.Error不为空
		case h.Error != "":
			call.Error = errors.New(h.Error)
			err = client.cc.ReadBody(nil)
			//调用call.done()通知调用方
			call.done()
		//正常处理
		default:
			//读取响应体
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			//调用call.done()通知调用方
			call.done()
		}
	}
	//出错，关闭连接
	client.terminateCalls(err)
}

//创建Client
func NewClient(conn net.Conn, opt *Option) (client *Client, err error) {
	//使用编解码器
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := errors.New("invalid codec type")
		_ = conn.Close()
		return nil, err
	}
	//发送Option，进行协议交换
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		_ = conn.Close()
		return nil, err
	}
	//创建Client
	clientt := &Client{
		seq:     1,
		cc:      f(conn),
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	//开启一个子协程接收响应
	go clientt.receive()
	return clientt, nil
}

//解析参数
func parseOptions(opts ...*Option) (*Option, error) {
	if len(opts) == 0 || opts[0] == nil {
		return DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}
	opt := opts[0]
	opt.MagicNumber = DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = DefaultOption.CodecType
	}
	return opt, nil
}

func dialTimeout(f newClientFunc, network, address string, opts ...*Option) (client *Client, err error) {
	//解析Option
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	//建立连接
	conn, err := net.DialTimeout(network, address, opt.ConnectTimeout)
	if err != nil {
		return nil, err
	}
	//关闭连接
	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()
	ch := make(chan clientResult)
	go func() {
		//创建Client
		client, err := f(conn, opt)
		//将结果发送到channel中
		ch <- clientResult{client: client, err: err}
	}()
	//设置连接超时
	if opt.ConnectTimeout == 0 {
		//没有设置连接超时，直接等待
		result := <-ch
		return result.client, result.err
	}
	//设置了连接超时，使用select实现
	select {
	//连接超时
	case <-time.After(opt.ConnectTimeout):
		return nil, errors.New("rpc client: connect timeout")
	//连接成功
	case result := <-ch:
		return result.client, result.err
	}
}

//解析参数（parseOptions），建立连接（net.Dial），创建Client（NewClient）
func Dial(network, address string, opts ...*Option) (*Client, error) {
	return dialTimeout(NewClient, network, address, opts...)
}

//发送请求
func (client *Client) send(call *Call) {
	//保证发送的请求是有序的
	client.sending.Lock()
	defer client.sending.Unlock()
	//注册请求
	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}
	//设置请求头
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""
	//发送请求
	if err := client.cc.Write(&client.header, call.Args); err != nil {
		//发送失败，从map中删除请求
		call := client.removeCall(seq)
		//通知请求方
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

//异步调用
func (client *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	//创建Call
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          make(chan *Call, 10),
	}
	//发送请求，异步调用，不需要等待响应
	client.send(call)
	return call
}

//同步调用
//使用context.WithTimeout()设置超时
// ctx, _ := context.WithTimeout(context.Background(), time.Second)
// var reply int
// err := client.Call(ctx, "Foo.Sum", &Args{1, 2}, &reply)
func (client *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	//创建Call，等待调用结束
	call := client.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select {
	case <-ctx.Done():
		//调用超时，从map中删除请求
		client.removeCall(call.Seq)
		//设置错误信息
		return errors.New("rpc client: call failed: " + ctx.Err().Error())
	case call := <-call.Done:
		//调用结束，返回错误信息
		return call.Error
	}
}

func NewHTTPClient(conn net.Conn, opt *Option) (*Client, error) {
	//发送HTTP协议的CONNECT请求
	_, _ = io.WriteString(conn, fmt.Sprintf("CONNECT %s HTTP/1.0\n\n", defaultRPCPath))

	//在转换成RPC协议之前，需要一个成功的HTTP响应
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err == nil && resp.Status == connected {
		//转换成RPC协议
		return NewClient(conn, opt)
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	return nil, err
}

func DialHTTP(network, address string, opts ...*Option) (*Client, error) {
	return dialTimeout(NewHTTPClient, network, address, opts...)
}

func XDial(rpcAddr string, opts ...*Option) (*Client, error) {
	parts := strings.Split(rpcAddr, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("rpc client err: wrong format '%s', expect protocol@addr", rpcAddr)
	}
	protocol, addr := parts[0], parts[1]
	switch protocol {
	case "http":
		return DialHTTP("tcp", addr, opts...)
	default:
		// tcp, unix or other transport protocol
		return Dial(protocol, addr, opts...)
	}
}

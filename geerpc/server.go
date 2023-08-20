package geerpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"geerpc/codec"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"
)

//标记是一个RPC请求
const MagicNumber = 0x3bef5c

const (
	//请求
	connected = "200 Connected to Gee RPC"
	//默认的RPC路径
	defaultRPCPath = "/_geerpc_"
	//默认的debug路径
	defaultDebugPath = "/debug/geerpc"
)

//一般来说，涉及协议协商的这部分信息，需要设计固定的字节来传输的。但是为了实现上更简单，GeeRPC 客户端固定采用 JSON 编码 Option，
//后续的 header 和 body 的编码方式由 Option 中的 CodeType 指定，服务端首先使用 JSON 解码 Option，
//然后通过 Option 的 CodeType 解码剩余的内容。即报文将以这样的形式发送：
//| Option{MagicNumber: xxx, CodecType: xxx} | Header{ServiceMethod ...} | Body interface{} |
//| <------      固定 JSON 编码      ------>  | <-------   编码方式由 CodeType 决定   ------->|

//RPC协议需要协商的内容，这里只有编码类型（其它的有压缩方式、长度信息等）
type Option struct {
	MagicNumber    int           //标记是一个RPC请求
	CodecType      codec.Type    //编码类型
	ConnectTimeout time.Duration //连接超时，默认是10，0表示不超时
	HandleTimeout  time.Duration //处理超时，默认是0
}

//默认的Option
var DefaultOption = &Option{
	MagicNumber:    MagicNumber,
	CodecType:      codec.GobType,
	ConnectTimeout: 10 * time.Second,
}

type Server struct {
	serviceMap sync.Map
}

func NewServer() *Server {
	return &Server{}
}

// DefaultServer 是一个默认的 Server 实例，主要为了用户使用方便
var DefaultServer = NewServer()

//lis, _ := net.Listen("tcp", ":9999")
func (server *Server) Accept(lis net.Listener) {
	//for 循环等待 socket 连接建立，并开启子协程处理，处理过程交给了 ServerConn 方法
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return
		}
		go server.ServeConn(conn)
	}
}

func Accept(lis net.Listener) { DefaultServer.Accept(lis) }

func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() { _ = conn.Close() }()
	var opt Option
	//解码Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: options error: ", err)
		return
	}
	//校验MagicNumber
	if opt.MagicNumber != MagicNumber {
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}
	//根据编码类型获取对应的编解码器
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", opt.CodecType)
		return
	}
	server.serveCodec(f(conn), &opt)
}

// invalidRequest is a placeholder for response argv when error occurs
var invalidRequest = struct{}{}

//主要包含了三个步骤：读取请求（readRequest），处理请求（handleRequest），回复请求（sendResponse）
func (server *Server) serveCodec(cc codec.Codec, opt *Option) {
	//确保发送完整的响应
	sending := new(sync.Mutex)
	//确保所有的请求都被处理完
	wg := new(sync.WaitGroup)
	//一个连接可以发送多个请求，所以这里使用了 for 循环
	for {
		req, err := server.readRequest(cc)
		if err != nil {
			if req == nil {
				break // it's not possible to recover, so close the connection
			}
			req.h.Error = err.Error()
			//发送错误响应
			server.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		//这里使用了 sync.WaitGroup 来确保所有的请求都被处理完
		wg.Add(1)
		//每个请求都开启一个子协程处理
		go server.handleRequest(cc, req, sending, wg, opt.HandleTimeout)
	}
	//等待所有的请求处理完毕后，关闭连接
	wg.Wait()
	_ = cc.Close()
}

// request stores all information of a call
type request struct {
	//请求头
	h *codec.Header
	//一个请求的参数和返回值都是 interface{} 类型
	argv, replyv reflect.Value // argv and replyv of request
	//请求的方法
	mtype *methodType // to get method's name and type
	//服务实例
	svc *service // to call the method
}

func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	//调用编解码器的读取请求头方法
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (server *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}
	req.svc, req.mtype, err = server.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReplyv()
	//确保请求的参数是指针类型
	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}
	if err = cc.ReadBody(argvi); err != nil {
		log.Println("rpc server: read body err:", err)
		return req, err
	}
	return req, nil
}

func (server *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	//响应的发送需要加锁，确保发送完整的响应
	sending.Lock()
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	//called 通道接收到消息，代表处理没有超时，继续执行sendResponse
	called := make(chan struct{})
	sent := make(chan struct{})
	go func() {
		err := req.svc.call(req.mtype, req.argv, req.replyv)
		called <- struct{}{}
		if err != nil {
			req.h.Error = err.Error()
			server.sendResponse(cc, req.h, invalidRequest, sending)
			sent <- struct{}{}
			return
		}
		//编码并发送响应
		server.sendResponse(cc, req.h, req.replyv.Interface(), sending)
		sent <- struct{}{}
	}()
	//处理超时
	if timeout == 0 {
		<-called
		<-sent
		return
	}
	//time.After(timeout)先于called接收到消息，代表处理超时，发送错误响应，called和sent都会被阻塞
	select {
	case <-time.After(timeout):
		req.h.Error = fmt.Sprintf("rpc server: request handle timeout: expect within %s", timeout)
		//发送错误响应
		server.sendResponse(cc, req.h, invalidRequest, sending)
	//处理完毕
	case <-called:
		//等待响应发送完毕
		<-sent
	}
}

func (server *Server) Register(rcvr interface{}) error {
	//通过反射获取 rcvr 的类型信息
	s := newService(rcvr)
	//检查服务名是否已经注册过
	if _, dup := server.serviceMap.LoadOrStore(s.name, s); dup {
		return errors.New("rpc: service already defined: " + s.name)
	}
	return nil
}

//服务端注册服务的过程，主要是将服务名和服务实例的映射关系存储到 serviceMap 中
func Register(rcvr interface{}) error { return DefaultServer.Register(rcvr) }

//通过服务名找到对应的服务实例
func (server *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	//解析服务名，获取服务名和方法名
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-formed: " + serviceMethod)
		return
	}
	//获取服务名
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	//通过服务名找到对应的服务实例
	svci, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}
	//类型断言，将 interface{} 类型转换为 *service 类型
	svc = svci.(*service)
	//通过方法名找到对应的方法
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("rpc server: can't find method " + methodName)
	}
	return
}

//实现了一个handler，用于处理RPC请求
func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	//必须是POST请求
	if req.Method != "CONNECT" {
		//返回一个错误响应
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = io.WriteString(w, "405 must CONNECT\n")
		return
	}
	// Hijack()方法用于接管底层的连接，返回一个 net.Conn 对象，该对象可以读写请求和响应
	// 这样就可以绕过 net/http 的请求处理逻辑，从而实现了一个简单的 RPC 服务端
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Print("rpc hijacking", req.RemoteAddr, ":", err.Error())
		return
	}
	//向客户端发送一个成功的响应，表示连接建立成功，后续的通信都将在这个连接上进行，而不是在 HTTP 连接上
	// 返回的响应格式为：HTTP/1.0 200 Connected to Gee RPC\n\n
	_, _ = io.WriteString(conn, "HTTP/1.0 "+connected+"\n\n")
	//处理连接
	server.ServeConn(conn)
}

// func (server *Server) HandleHTTP() {
// 	http.Handle(defaultRPCPath, server)
// }

func (server *Server) HandleHTTP() {
	//注册一个HTTP处理器，用于处理RPC请求
	http.Handle(defaultRPCPath, server)
	http.Handle(defaultDebugPath, debugHTTP{server})
	log.Println("rpc server debug path:", defaultDebugPath)
}

// 一个方便默认服务器注册 HTTP 处理器的方法
func HandleHTTP() {
	DefaultServer.HandleHTTP()
}

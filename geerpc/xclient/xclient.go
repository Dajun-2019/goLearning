package xclient

import (
	//引入geerpc包，因为要用到geerpc包中的一些结构体
	"context"
	. "geerpc"
	"io"
	"reflect"
	"sync"
)

type XClient struct {
	d       Discovery
	mode    SelectMode
	opt     *Option
	mu      sync.Mutex //protecting following
	clients map[string]*Client
}

var _ io.Closer = (*XClient)(nil)

//创建一个XClient实例
func NewXClient(d Discovery, mode SelectMode, opt *Option) *XClient {
	return &XClient{
		d:       d,
		mode:    mode,
		opt:     opt,
		clients: make(map[string]*Client),
	}
}

func (xc *XClient) Close() error {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	//关闭所有的Client实例
	for key, client := range xc.clients {
		_ = client.Close()
		delete(xc.clients, key)
	}
	return nil
}

//根据服务名，返回一个Client实例，检查xc.clients中是否已经缓存了对应的Client实例
func (xc *XClient) dial(rpcAddr string) (*Client, error) {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	//先查看xc.clients中是否已经有了对应的Client实例
	client, ok := xc.clients[rpcAddr]
	if ok && !client.IsAvailable() {
		//如果有，但是不可用，则关闭该实例
		_ = client.Close()
		delete(xc.clients, rpcAddr)
		client = nil
	}
	if client == nil {
		//如果没有，则创建一个Client实例
		var err error
		client, err = XDial(rpcAddr, xc.opt)
		if err != nil {
			return nil, err
		}
		//将新创建的Client实例存入xc.clients
		xc.clients[rpcAddr] = client
	}
	return client, nil
}

//根据负载均衡策略，选择一个服务实例，返回一个Client实例
func (xc *XClient) call(rpcAddr string, ctx context.Context, serviceMethod string, args, reply interface{}) error {
	//根据服务名，获取一个Client实例
	client, err := xc.dial(rpcAddr)
	if err != nil {
		return err
	}
	//调用Client实例的Call方法
	return client.Call(ctx, serviceMethod, args, reply)
}

func (xc *XClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	//根据负载均衡策略，选择一个服务实例，返回一个rpc地址
	rpcAddr, err := xc.d.Get(xc.mode)
	if err != nil {
		return err
	}
	return xc.call(rpcAddr, ctx, serviceMethod, args, reply)
}

//将请求广播到所有的服务实例，只要有一个服务实例调用成功，就返回
func (xc *XClient) Broadcast(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	//获取所有的服务实例
	servers, err := xc.d.GetAll()
	if err != nil {
		return err
	}
	//创建一个错误通道
	var wg sync.WaitGroup
	var mu sync.Mutex //保护 e 和 replyDone
	var e error
	//遍历所有的服务实例
	replyDone := reply == nil //如果reply为nil，说明不需要接收reply
	//借助 context.WithCancel 确保有错误发生时，快速失败
	ctx, cancel := context.WithCancel(ctx)
	for _, rpcAddr := range servers {
		//为每个服务实例创建一个goroutine
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var clonedReply interface{}
			if reply != nil {
				//如果需要接收reply，则创建一个新的reply
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			//调用call方法
			err := xc.call(rpcAddr, ctx, serviceMethod, args, clonedReply)
			mu.Lock()
			if err != nil && e == nil {
				//如果调用call方法失败，则将错误保存在e中
				e = err
				cancel() //if any call failed, cancel unfinished calls
			}
			if err == nil && !replyDone {
				//如果调用call方法成功，且replyDone为false，则将reply保存在reply中
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
				replyDone = true
			}
			mu.Unlock()
		}(rpcAddr)
	}
	//等待所有的goroutine执行完毕
	wg.Wait()
	return e
}

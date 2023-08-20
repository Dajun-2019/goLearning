package main

import (
	"context"
	"geerpc"
	"geerpc/registry"
	"geerpc/xclient"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type Foo int

type Args struct {
	Num1, Num2 int
}

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo) Sleep(args Args, reply *int) error {
	time.Sleep(time.Second * time.Duration(args.Num1))
	*reply = args.Num1 + args.Num2
	return nil
}

// func startServer(addrCh chan string) {
// 	var foo Foo
// 	l, _ := net.Listen("tcp", ":0")
// 	server := geerpc.NewServer()
// 	_ = server.Register(&foo)
// 	addrCh <- l.Addr().String()
// 	server.Accept(l)
// }

//便于在 Call 或 Broadcast 之后统一打印成功或失败的日志
func foo(xc *xclient.XClient, ctx context.Context, typ, serviceMethod string, args *Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil {
		log.Printf("%s %s error: %v", typ, serviceMethod, err)
	} else {
		log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod, args.Num1, args.Num2, reply)
	}
}

//调用单个服务实例
func call(registry string) {
	// d := xclient.NewMultiServerDiscovery([]string{"tcp@" + addr1, "tcp@" + addr2})
	d := xclient.NewGeeRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() { _ = xc.Close() }()
	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

//调用所有的服务实例
func broadcast(registry string) {
	// d := xclient.NewMultiServerDiscovery([]string{"tcp@" + addr1, "tcp@" + addr2})
	d := xclient.NewGeeRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() { _ = xc.Close() }()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "broadcast", "Foo.Sum", &Args{Num1: i, Num2: i * i})
			// expect 2 - 5 timeout
			ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
			foo(xc, ctx, "broadcast", "Foo.Sleep", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func startRegistry(wg *sync.WaitGroup) {
	l, _ := net.Listen("tcp", ":9999")
	registry.HandleHTTP()
	wg.Done()
	_ = http.Serve(l, nil)
}

func startServer(registryAddr string, wg *sync.WaitGroup) {
	var foo Foo
	l, _ := net.Listen("tcp", ":0")
	server := geerpc.NewServer()
	_ = server.Register(&foo)
	registry.Heartbeat(registryAddr, "tcp@"+l.Addr().String(), 0)
	wg.Done()
	server.Accept(l)
}

func main() {
	// addr := make(chan string)
	// go startServer(addr)

	// // in fact, following code is like a simple geerpc client
	// conn, _ := net.Dial("tcp", <-addr)
	// defer func() { _ = conn.Close() }()

	// time.Sleep(time.Second)
	// // send options
	// _ = json.NewEncoder(conn).Encode(geerpc.DefaultOption)
	// cc := codec.NewGobCodec(conn)
	// // send request & receive response
	// for i := 0; i < 5; i++ {
	// 	h := &codec.Header{
	// 		ServiceMethod: "Foo.Sum",
	// 		Seq:           uint64(i),
	// 	}
	// 	_ = cc.Write(h, fmt.Sprintf("geerpc req %d", h.Seq))
	// 	_ = cc.ReadHeader(h)
	// 	var reply string
	// 	_ = cc.ReadBody(&reply)
	// 	log.Println("reply:", reply)
	// }

	// log.SetFlags(0)
	// //chan string 用于传递地址，chan<- string 用于发送地址，<-chan string 用于接收地址
	// //这里的语法chan string是一个无缓冲的通道，用于传递地址
	// addr := make(chan string)
	// //开启一个子协程启动服务端
	// go startServer(addr)
	// client, _ := geerpc.Dial("tcp", <-addr)
	// defer func() { _ = client.Close() }()

	// time.Sleep(time.Second)
	// // send request & receive response
	// var wg sync.WaitGroup
	// for i := 0; i < 5; i++ {
	// 	wg.Add(1)
	// 	go func(i int) {
	// 		defer wg.Done()
	// 		args := &Args{Num1: i, Num2: i * i}
	// 		// args := fmt.Sprintf("geerpc req %d", i)
	// 		var reply int
	// 		//调用客户端的Call方法
	// 		if err := client.Call("Foo.Sum", args, &reply); err != nil {
	// 			log.Fatal("call Foo.Sum error:", err)
	// 		}
	// 		log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
	// 		// log.Println("reply:", reply)
	// 	}(i)
	// }
	// //等待所有的请求处理完毕后，关闭连接
	// wg.Wait()
	// 	log.SetFlags(0)
	// 	ch := make(chan string)
	// 	go call(ch)
	// 	startServer(ch)
	// }

	// log.SetFlags(0)
	// ch1 := make(chan string)
	// ch2 := make(chan string)
	// // start two servers
	// go startServer(ch1)
	// go startServer(ch2)

	// addr1 := <-ch1
	// addr2 := <-ch2

	// time.Sleep(time.Second)
	// call(addr1, addr2)
	// broadcast(addr1, addr2)
	log.SetFlags(0)
	registryAddr := "http://localhost:9999/_geerpc_/registry"
	var wg sync.WaitGroup
	wg.Add(1)
	go startRegistry(&wg)
	wg.Wait()

	time.Sleep(time.Second)
	wg.Add(2)
	go startServer(registryAddr, &wg)
	go startServer(registryAddr, &wg)
	wg.Wait()

	time.Sleep(time.Second)
	call(registryAddr)
	broadcast(registryAddr)
}

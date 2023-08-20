package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type GeeRegistry struct {
	// timeout: 服务列表的过期时间，0表示永不过期
	timeout time.Duration
	mu      sync.Mutex
	servers map[string]*ServerItem
}

type ServerItem struct {
	Addr  string
	start time.Time
}

const (
	defaultPath = "/_geerpc_/registry"
	// 默认超时时间，5分钟
	defaultTimeout = time.Minute * 5
)

func New(timeout time.Duration) *GeeRegistry {
	return &GeeRegistry{
		servers: make(map[string]*ServerItem),
		timeout: timeout,
	}
}

var DefaultGeeRegistry = New(defaultTimeout)

// putServer: 注册服务
func (r *GeeRegistry) putServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.servers[addr]
	if s == nil {
		// 新增服务，添加到服务列表
		r.servers[addr] = &ServerItem{Addr: addr, start: time.Now()}
	} else {
		// 更新服务，更新服务的时间
		s.start = time.Now()
	}
}

// aliveServers: 获取可用服务
func (r *GeeRegistry) aliveServers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	// 遍历所有服务，删除超时的服务，添加存活的服务
	var alive []string
	for addr, s := range r.servers {
		//s.start.Add(r.timeout).After(time.Now())表示服务的过期时间大于当前时间
		if r.timeout == 0 || s.start.Add(r.timeout).After(time.Now()) {
			// 添加存活的服务
			alive = append(alive, addr)
		} else {
			// 删除超时的服务
			delete(r.servers, addr)
		}
	}
	// 排序
	sort.Strings(alive)
	return alive
}

// GeeRegistry采用HTTP协议作为注册中心的通信协议，因此需要提供HTTP的接口
// GET 返回所有可用的服务，通过自定义字段 X-Geerpc-Servers 承载
// POST 添加服务实例或发送心跳，通过自定义字段 X-Geerpc-Server 承载
func (r *GeeRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		// GET 返回所有可用的服务
		w.Header().Set("X-Geerpc-Servers", strings.Join(r.aliveServers(), ","))
	case "POST":
		// 从请求头中获取服务实例的地址
		addr := req.Header.Get("X-Geerpc-Server")
		if addr == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// 添加服务实例或发送心跳
		r.putServer(addr)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *GeeRegistry) HandleHTTP(registryPath string) {
	http.Handle(registryPath, r)
	log.Println("rpc registry path:", registryPath)
}

func HandleHTTP() {
	DefaultGeeRegistry.HandleHTTP(defaultPath)
}

func Heartbeat(registry, addr string, duration time.Duration) {
	if duration == 0 {
		// 确保有足够的时间发送心跳
		duration = defaultTimeout - time.Duration(1)*time.Minute
	}
	var err error
	err = sendHeartbeat(registry, addr)
	// 定时发送心跳，如果发送失败，就一直重试，直到发送成功
	go func() {
		// Ticker是一个定时触发的计时器，它会以一个间隔(interval)往channel发送一个事件(当前时间)，
		t := time.NewTicker(duration)
		for err == nil {
			// 等待下一个心跳时间
			<-t.C
			// 发送心跳
			err = sendHeartbeat(registry, addr)
		}
	}()
}

func sendHeartbeat(registry, addr string) error {
	log.Println(addr, "send heart beat to registry", registry)
	httpClient := &http.Client{}
	// 构造请求
	req, _ := http.NewRequest("POST", registry, nil)
	// 添加自定义字段
	req.Header.Set("X-Geerpc-Server", addr)
	// 发送请求
	if _, err := httpClient.Do(req); err != nil {
		log.Println("rpc server: heart beat err:", err)
		return err
	}
	return nil
}

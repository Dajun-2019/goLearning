package xclient

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

//代表不同的负载均衡策略
type SelectMode int

const (
	RandomSelect     SelectMode = iota //随机选择
	RoundRobinSelect                   //轮询
)

type Discovery interface {
	Refresh() error                      //从注册中心更新服务列表
	Update(servers []string) error       //手动更新服务列表
	Get(mode SelectMode) (string, error) //根据负载均衡策略，选择一个服务实例
	GetAll() ([]string, error)           //返回所有的服务实例
}

//一个不需要注册中心，服务列表是手动维护的服务发现的结构体
type MultiServersDiscovery struct {
	r       *rand.Rand   //产生随机数
	mu      sync.RWMutex //用于更新服务列表时的互斥锁
	servers []string     //服务实例列表
	index   int          //记录上次选择的服务实例的下标
}

func NewMultiServerDiscovery(servers []string) *MultiServersDiscovery {
	d := &MultiServersDiscovery{
		servers: servers,
		//初始化随机数生成器，使用当前时间作为随机数种子
		r: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	d.index = d.r.Intn(math.MaxInt32 - 1) //随机初始化index，避免每次都从0开始
	return d
}

//保证MultiServersDiscovery实现了Discovery接口
var _ Discovery = (*MultiServersDiscovery)(nil)

func (d *MultiServersDiscovery) Refresh() error {
	return nil
}

func (d *MultiServersDiscovery) Update(servers []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers
	return nil
}

// func ErrInvalidSelectMode() error {
// 	return errors.New("rpc discovery: invalid select mode")
// }

func (d *MultiServersDiscovery) Get(mode SelectMode) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	//如果服务列表为空，返回空字符串
	n := len(d.servers)
	if n == 0 {
		return "", errors.New("rpc discovery: no available servers")
	}
	//根据负载均衡策略，选择一个服务实例
	switch mode {
	case RandomSelect:
		return d.servers[d.r.Intn(n)], nil
	case RoundRobinSelect:
		s := d.servers[d.index%n]
		d.index = (d.index + 1) % n
		return s, nil
	default:
		return "", errors.New("rpc discovery: not supported select mode")
	}
}

func (d *MultiServersDiscovery) GetAll() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	//返回所有的服务实例
	servers := make([]string, len(d.servers), len(d.servers))
	copy(servers, d.servers)
	return servers, nil
}

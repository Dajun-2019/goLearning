package xclient

import (
	"log"
	"net/http"
	"strings"
	"time"
)

type GeeRegistryDiscovery struct {
	// 嵌套了一个MultiServersDiscovery，这样GeeRegistryDiscovery就拥有了MultiServersDiscovery的所有方法
	*MultiServersDiscovery
	registry   string        // 注册中心地址
	timeout    time.Duration // 超时时间
	lastUpdate time.Time     // 最后更新时间，默认10s过期，即10s后重新从注册中心更新服务列表
}

const defaultUpdateTimeout = time.Second * 10

func NewGeeRegistryDiscovery(registerAddr string, timeout time.Duration) *GeeRegistryDiscovery {
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}
	d := &GeeRegistryDiscovery{
		MultiServersDiscovery: NewMultiServerDiscovery(make([]string, 0)),
		registry:              registerAddr,
		timeout:               timeout,
	}
	return d
}

// Update: 手动更新服务列表
func (d *GeeRegistryDiscovery) Update(servers []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers
	d.lastUpdate = time.Now()
	return nil
}

// Refresh: 从注册中心更新服务列表
func (d *GeeRegistryDiscovery) Refresh() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	// 判断是否需要更新
	if d.lastUpdate.Add(d.timeout).After(time.Now()) {
		return nil
	}
	log.Println("rpc registry: refresh servers from registry", d.registry)
	// 从注册中心获取服务列表
	resp, err := http.Get(d.registry)
	if err != nil {
		log.Println("rpc registry refresh err:", err)
		return err
	}
	servers := strings.Split(resp.Header.Get("X-Geerpc-Servers"), ",")
	d.servers = make([]string, 0, len(servers))
	for _, server := range servers {
		// 去除空格
		if strings.TrimSpace(server) != "" {
			// 添加到服务列表
			d.servers = append(d.servers, strings.TrimSpace(server))
		}
	}
	// 更新最后更新时间
	d.lastUpdate = time.Now()
	return nil
}

// Get: 从注册中心获取服务列表
func (d *GeeRegistryDiscovery) Get(mode SelectMode) (string, error) {
	// 判断是否需要更新
	if err := d.Refresh(); err != nil {
		return "", err
	}
	// 从服务列表中选择一个服务
	return d.MultiServersDiscovery.Get(mode)
}

// GetAll: 从注册中心获取服务列表
func (d *GeeRegistryDiscovery) GetAll() ([]string, error) {
	// 判断是否需要更新
	if err := d.Refresh(); err != nil {
		return nil, err
	}
	// 返回所有服务
	return d.MultiServersDiscovery.GetAll()
}

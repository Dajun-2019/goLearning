package geecache

import (
	pb "geecache/geecachepb"
	"geecache/singleflight"
	"log"
	"sync"
)

type Getter interface {
	Get(key string) ([]byte, error)
}

type GetterFunc func(key string) ([]byte, error)

// 实现了Getter接口的Get方法
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

//看做一个缓存的命名空间
type Group struct {
	name string
	//缓存未命中时的回调函数
	getter Getter
	//缓存
	mainCache cache
	peers     PeerPicker
	loader    *singleflight.Group
}

//全局变量
var (
	mu sync.RWMutex
	//全局变量，用来存储所有的Group，通过name来获取
	groups = make(map[string]*Group)
)

func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	group := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
	}
	groups[name] = group
	return group
}

func GetGroup(name string) *Group {
	mu.RLock()
	// 从全局变量中获取指定名称的Group
	g := groups[name]
	mu.RUnlock()
	return g
}

func (g *Group) Get(key string) (ByteView, error) {
	//空key
	if key == "" {
		return ByteView{}, nil
	}
	//从缓存中获取
	if v, ok := g.mainCache.get(key); ok {
		return v, nil
	}
	//缓存未命中，调用load方法，载入数据
	return g.load(key)
}

// func (g *Group) load(key string) (value ByteView, err error) {
// 	//通过调用用户回调函数，来获取数据，而不是提前规定好所有的数据源
// 	//这里使用的是本地的方式，后面会更改为分布式的方式，调用getFromPeer从其它节点获取
// 	return g.getLocally(key)
// }

func (g *Group) load(key string) (value ByteView, err error) {
	//使用Do方法，确保每个key只被请求一次
	viewi, err := g.loader.Do(key, func() (interface{}, error) {

		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok {
				//从远程节点获取
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				//远程节点没有，则可能是本机节点，或者缓存失效，从本机获取调用getLocally来验证
				log.Println("[GeeCache] Failed to get from peer", err)
			}
		}
		return g.getLocally(key)
	})
	if err == nil {
		//类型断言
		return viewi.(ByteView), nil
	}
	return
}

//分布式环境下会调用getFromPeer从其他节点获取缓存
func (g *Group) getLocally(key string) (ByteView, error) {
	//获取并调用用户回调函数
	bytes, err := g.getter.Get(key)
	if err != nil {
		//没有对应数据
		return ByteView{}, err
	}
	//使用ByteView封装数据值
	value := ByteView{b: cloneBytes(bytes)}
	//将缓存值添加到缓存中
	g.populateCache(key, value)
	return value, nil
}

//将数据添加到缓存中
func (g *Group) populateCache(key string, value ByteView) {
	//将缓存值添加到缓存中
	g.mainCache.add(key, value)
}

func (g *Group) RegisterPeers(peers PeerPicker) {
	//如果已经注册过了，就panic
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	//注册Peer，一个Group只能注册一次，即一个Group只能有一个PeerPicker
	g.peers = peers
}

//从远程节点获取
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	//创建一个字节切片，用来存储获取到的数据
	//本地直接get，这里要使用peer的get方法，使用http客户端
	// bytes, err := peer.Get(g.name, key)
	// if err != nil {
	// 	return ByteView{}, err
	// }
	// //使用ByteView封装数据值
	// value := ByteView{b: cloneBytes(bytes)}
	// //将缓存值添加到本地缓存中
	// g.populateCache(key, value)
	// return value, nil
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: res.Value}, nil
}

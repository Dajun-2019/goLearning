/*
建立节点之间的通信，通过http访问其它节点
*/
package geecache

import (
	"fmt"
	"geecache/consistenthash"
	pb "geecache/geecachepb"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"
)

//节点之间通信地址的前缀，具有这个前缀的请求用于节点之间的访问
const defaultBasePath = "/_geecache/"
const defaultReplicas = 50

//用来承载节点之间的HTTP通信的核心数据结构
type HTTPPool struct {
	//用来记录自己的地址
	self string
	// 作为节点间通讯地址的前缀，默认是 /_geecache/，那么 http://example.com/_geecache/ 开头的请求，
	// 就用于节点间的访问。为了避免与用户的请求冲突，约定访问节点间通讯地址的前缀默认添加 /_geecache/ 前缀
	basePath string
	mu       sync.Mutex
	//节点间通讯地址的map，key是具体的节点的地址，value是对应节点的httpGetter，每一个httpGetter对应一个远程节点
	httpGetters map[string]*httpGetter
	//根据具体的key选择节点的一致性哈希算法的实例
	peers *consistenthash.Map
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// Log info with server name
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

//约定访问路径格式为 /<basepath>/<groupname>/<key>
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 判断前缀是否正确
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	//记录日志
	p.Log("%s %s", r.Method, r.URL.Path)
	//使用/分割url，只要分割出来3部分，就停止，从groupName开始分割，前面通过切片跳过了
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	//获取group和key
	groupName := parts[0]
	key := parts[1]
	//获取当前节点下的groupName对应的group
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}
	//获取需要的key的缓存值，如果没有就返回error
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write the value to the response body as a proto message.
	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//将缓存值写入到ResponseWriter
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(body)
}

type httpGetter struct {
	//用来访问远程节点的地址，http://example.com/_geecache/
	baseURL string
}

//实现了PeerGetter接口的Get方法，用来访问远程节点
// func (h *httpGetter) Get(group string, key string) ([]byte, error) {
func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	//拼接访问远程节点的URL
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		//转义字符串，以便可以放置在URL查询中
		url.QueryEscape(in.GetGroup()),
		url.QueryEscape(in.GetKey()),
	)
	//发起http请求
	res, err := http.Get(u)
	if err != nil {
		return err
	}
	//关闭请求
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}
	//读取结果
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}

	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}
	//返回结果
	return nil

}

func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	//初始化一致性哈希算法
	p.peers = consistenthash.New(defaultReplicas, nil)
	//添加传入的节点
	p.peers.Add(peers...)
	//初始化httpGetters
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	//为每一个节点创建一个httpGetter
	for _, peer := range peers {
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

//根据具体的key选择节点，返回对应的httpGetter
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	//根据具体的key选择节点
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		//返回对应的httpGetter
		return p.httpGetters[peer], true
	}
	return nil, false
}

var _ PeerPicker = (*HTTPPool)(nil)

package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func(data []byte) uint32

//保存所有的keys
type Map struct {
	//哈希函数
	hash Hash
	//虚拟节点的个数
	replicas int
	//所有key，有序的
	keys []int
	//虚拟节点与真实节点的映射关系
	hashmap map[int]string
}

//创建一个Map
func New(relicas int, fn Hash) *Map {
	m := &Map{
		hash:     fn,
		replicas: relicas,
		hashmap:  make(map[int]string),
	}
	if m.hash == nil {
		//默认实现，返回uint32
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

//允许传入0个或多个真实节点的名称
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			//0key 1key 2key 3key ...
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash)
			m.hashmap[hash] = key
		}
	}
	sort.Ints(m.keys)
}

//选择合适的节点
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}
	hash := int(m.hash([]byte(key)))
	//使用二分查找找到第一个匹配（func(i)返回true）的虚拟节点的下标idx
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	//如果idx==len(m.keys)，说明应选择m.keys[0]，因为m.keys是一个环状结构，所以用取余数的方式
	return m.hashmap[m.keys[idx%len(m.keys)]]
}

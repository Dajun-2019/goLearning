package lru

import "container/list"

type Cache struct {
	//cache允许最大内存
	maxBytes int64
	//cache当前已使用的内存
	nbytes int64
	//标准库实现的双向链表
	ll    *list.List               //双向链表
	cache map[string]*list.Element //字典
	// 可选，当条目被清除时执行
	OnEvicted func(key string, value Value)
}

// 双向链表节点的数据类型
type entry struct {
	key   string
	value Value
}

// Value 用于计算一个条目的内存大小
// 链表的值是实现了Value接口的任意类型
type Value interface {
	// 返回值所占用的内存大小
	Len() int
}

func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,                       //1k*1k*1k = 1G
		ll:        list.New(),                     //初始化双向链表
		cache:     make(map[string]*list.Element), //初始化字典
		OnEvicted: onEvicted,
	}
}

// 查找功能
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, true
	}
	return
}

// 删除
func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		//从字典中删除
		delete(c.cache, kv.key)
		//更新当前所用内存
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.OnEvicted != nil {
			//调用回调函数
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// 添加
func (c *Cache) Add(key string, value Value) {
	//如果键存在，则更新对应节点的值，并将该节点移到队首
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		//更新当前所用内存
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		//不存在则添加到队首
		ele := c.ll.PushFront(&entry{key, value})
		//更新字典
		c.cache[key] = ele
		//更新当前所用内存
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	//如果超过了设定的最大内存，则移除最少访问的节点
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.RemoveOldest()
	}
}

func (c *Cache) Len() int {
	return c.ll.Len()
}

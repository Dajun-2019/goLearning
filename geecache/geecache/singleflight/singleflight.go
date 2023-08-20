package singleflight

import "sync"

//正在进行中的，或已经结束的请求
type call struct {
	// wg 用于实现并发控制，避免重入
	wg sync.WaitGroup
	// val 用于保存请求的结果
	val interface{}
	// err 用于保存请求的错误信息
	err error
}

//主要数据结构，管理不同key的请求(call)
type Group struct {
	mu sync.Mutex
	//用来记录不同key的请求(call)
	m map[string]*call
}

//针对相同的key，不论Do被调用多少次，函数fn都只会被调用一次，等待fn调用结束了，返回返回值或错误
//保存在map，每次的新查询都会查找，找到则返回，否则就新建一个call，调用fn，然后保存到map中
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		// 第一个get(key)请求到来时，singleflight会记录当前key正在被处理，
		// 后续的请求只需要等待第一个请求处理完成，取返回值即可
		c.wg.Wait()
		return c.val, c.err
	}
	c := new(call)
	c.wg.Add(1)  //发起请求前加锁
	g.m[key] = c //添加到 g.m，表明 key 已经有对应的请求在处理
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done() //请求结束

	g.mu.Lock()
	//删除 g.m 中的记录，因为数据会被更新，后续的请求需要重新请求
	delete(g.m, key) //更新 g.m
	g.mu.Unlock()

	return c.val, c.err
}

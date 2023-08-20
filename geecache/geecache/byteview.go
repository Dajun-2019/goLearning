package geecache

// ByteView 是一个只读的 byte 类型的视图，用来表现缓存值

type ByteView struct {
	b []byte
}

// lru.Value 接口的实现，返回所占用的内存大小
func (v ByteView) Len() int {
	return len(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

func (v ByteView) String() string {
	return string(v.b)
}

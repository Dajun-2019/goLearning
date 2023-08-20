package geecache

import pb "geecache/geecachepb"

//通过key找到对应的PeerGetter，使用一致性哈希算法
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

//从对应的group和对应的key找到对应的值，使用http客户端
// type PeerGetter interface {
// 	Get(group string, key string) ([]byte, error)
// }

type PeerGetter interface {
	Get(in *pb.Request, out *pb.Response) error
}

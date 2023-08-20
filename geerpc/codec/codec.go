//客户端发送的请求包括服务名 Arith，方法名 Multiply，参数 args 三个，服务端的响应包括错误 error，返回值 reply 2 个
//err = client.Call("Arith.Multiply", args, &reply) 即服务名.方法名

package codec

import "io"

type Header struct {
	ServiceMethod string // format "Service.Method"，即GO语言中的结构体名.方法名
	Seq           uint64 // 客户选择的序列号，可以用来区分不同的请求
	Error         string
}

//消息的编解码器，后续可以扩展其他编码器
type Codec interface {
	//关闭连接
	io.Closer
	//读取请求头
	ReadHeader(*Header) error
	//读取请求体
	ReadBody(interface{}) error
	//写入请求头
	Write(*Header, interface{}) error
}

//编解码器的构造函数
type NewCodecFunc func(io.ReadWriteCloser) Codec

//编解码器的类型
type Type string

//定义了两种编解码器
const (
	GobType  Type = "application/gob"  //gob编码
	JsonType Type = "application/json" //json编码
)

//编解码器的构造函数映射表
var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
}

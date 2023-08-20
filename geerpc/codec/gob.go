package codec

import (
	"bufio"

	//官方包 用于编解码
	"encoding/gob"
	"io"
	"log"
)

type GobCodec struct {
	//连接
	conn io.ReadWriteCloser //gob编码的连接
	buf  *bufio.Writer      //缓冲区
	dec  *gob.Decoder       //解码器
	enc  *gob.Encoder       //编码器
}

//关闭连接
//这种写法用于清晰的确定GobCodec实现了Codec接口，一个类型断言用来检查接口类型GobCodec是否实现了Codec接口，如果没实现则会产生一个运行时错误
var _ Codec = (*GobCodec)(nil)

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf, //缓冲区
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}

func (c *GobCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h)
}

func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *GobCodec) Write(h *Header, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()
	//编码器编码 header
	if err := c.enc.Encode(h); err != nil {
		log.Println("rpc codec: gob error encoding header:", err)
		return err
	}
	//编码器编码 body
	if err := c.enc.Encode(body); err != nil {
		log.Println("rpc codec: gob error encoding body:", err)
		return err
	}
	return nil
}

func (c *GobCodec) Close() error {
	return c.conn.Close()
}

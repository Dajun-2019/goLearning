package geerpc

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

type methodType struct {
	method    reflect.Method //方法本身
	ArgType   reflect.Type   //第一个参数的类型
	ReplyType reflect.Type   //第二个参数的类型
	numCalls  uint64         //统计方法调用次数
}

type service struct {
	name   string                 //服务名
	typ    reflect.Type           //服务类型
	rcvr   reflect.Value          //服务实例本身，在调用方法时，需要传入实例本身作为第0个参数
	method map[string]*methodType //服务中所有的方法
}

func (m *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls)
}

func (m *methodType) newArgv() reflect.Value {
	//创建一个新的参数实例
	var argv reflect.Value
	//如果参数是指针类型
	if m.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(m.ArgType.Elem())
	} else {
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

func (m *methodType) newReplyv() reflect.Value {
	replyv := reflect.New(m.ReplyType.Elem())
	//需要传入指针类型
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		//如果是map类型，需要初始化
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		//如果是切片类型，需要初始化
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return replyv
}

func newService(rcvr interface{}) *service {
	s := new(service)
	//获取服务实例的反射值
	s.rcvr = reflect.ValueOf(rcvr)
	//获取服务实例的名称
	s.name = reflect.Indirect(s.rcvr).Type().Name()
	//获取服务实例的类型
	s.typ = reflect.TypeOf(rcvr)
	if !ast.IsExported(s.name) {
		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}
	//检查服务实例的合法性
	s.registerMethods()
	return s
}

func (s *service) registerMethods() {
	s.method = make(map[string]*methodType)
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mType := method.Type
		//需要满足的条件：方法的参数个数为3，第一个参数是接收者本身，第二个参数是输入参数，第三个参数是输出参数，输出参数必须是指针类型
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		//输出参数必须是error类型
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		//输入参数必须是导出或内置类型
		argType, replyType := mType.In(1), mType.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}
		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s\n", s.name, method.Name)
	}
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	//内置类型,不需要导出,直接返回true,比如int,string等
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

//通过反射调用方法
func (s *service) call(m *methodType, argv, replyv reflect.Value) error {
	//统计方法调用次数
	atomic.AddUint64(&m.numCalls, 1)
	//调用方法
	function := m.method.Func
	//传入参数
	returnValues := function.Call([]reflect.Value{s.rcvr, argv, replyv})
	//获取错误信息
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

//实现session的钩子函数
package session

import "reflect"

const (
	BeforeQuery = "BeforeQuery"
	AfterQuery  = "AfterQuery"
	BeforeExec  = "BeforeExec"
	AfterExec   = "AfterExec"

	BeforeDelete = "BeforeDelete"
	AfterDelete  = "AfterDelete"
	BeforeUpdate = "BeforeUpdate"
	AfterUpdate  = "AfterUpdate"
	BeforeInsert = "BeforeInsert"
	AfterInsert  = "AfterInsert"
)

func (s *Session) CallMethod(method string, value interface{}) {
	// s.RefTable().Model 或 value 即当前会话正在操作的对象
	// 获取被映射对象的反射值
	fm := reflect.ValueOf(s.RefTable().Model).MethodByName(method)
	// 如果被映射对象的反射值不为空，则调用该方法
	if value != nil {
		//获取参数的反射值
		fm = reflect.ValueOf(value).MethodByName(method)
	}
	//获取参数
	param := []reflect.Value{reflect.ValueOf(s)}
	if fm.IsValid() {
		//调用方法
		if v := fm.Call(param); len(v) > 0 {
			if err, ok := v[0].Interface().(error); ok {
				panic(err)
			}
		}
	}
	// return
}

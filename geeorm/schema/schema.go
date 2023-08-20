package schema

import (
	"geeorm/dialect"
	"reflect"
)

type Field struct {
	//字段名
	Name string
	//字段类型
	Type string
	//字段约束
	Tag string
}

type Schema struct {
	//被映射的对象
	Model interface{}
	//表名
	Name string
	//所有的字段
	Fields []*Field
	//所有的字段名
	FieldNames []string
	//字段名和Field的映射
	fieldMap map[string]*Field
}

//通过反射将dest对象解析为Schema对象
func Parse(dest interface{}, d dialect.Dialect) *Schema {
	//获取被映射对象的类型
	//TypeOf() 和 ValueOf() 分别用来返回入参的类型和值
	//因为设计的入参是一个对象的指针，因此需要 reflect.Indirect() 获取指针指向的实例
	modelType := reflect.Indirect(reflect.ValueOf(dest)).Type()
	//创建Schema对象
	schema := &Schema{
		Model:    dest,
		Name:     modelType.Name(),
		fieldMap: make(map[string]*Field),
	}
	//遍历结构体的字段
	for i := 0; i < modelType.NumField(); i++ {
		//获取每个字段的反射值
		p := modelType.Field(i)
		//判断字段是否为匿名字段,匿名字段不做处理,因为匿名字段不会生成表的字段,也不会生成表,因此不需要处理,直接跳过
		if !p.Anonymous && p.PkgPath == "" {
			//创建Field对象
			field := &Field{
				Name: p.Name,
				Type: d.DataTypeOf(reflect.Indirect(reflect.New(p.Type))),
			}
			//获取字段的tag
			//Name string `geeorm:"PRIMARY KEY"`
			if v, ok := p.Tag.Lookup("geeorm"); ok {
				field.Tag = v
			}
			schema.Fields = append(schema.Fields, field)
			schema.FieldNames = append(schema.FieldNames, p.Name)
			schema.fieldMap[p.Name] = field
		}
	}
	return schema
}

//根据字段名获取字段
func (schema *Schema) GetField(name string) *Field {
	return schema.fieldMap[name]
}

//获取每一次插入的值，把插入的值放入到一个切片中
func (schema *Schema) RecordValues(dest interface{}) []interface{} {
	destValue := reflect.Indirect(reflect.ValueOf(dest))
	var fieldValues []interface{}
	//遍历字段
	for _, field := range schema.Fields {
		//获取字段的值
		fieldValues = append(fieldValues, destValue.FieldByName(field.Name).Interface())
	}
	return fieldValues
}

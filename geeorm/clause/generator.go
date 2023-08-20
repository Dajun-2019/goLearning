package clause

import (
	"fmt"
	"strings"
)

//通过values参数，生成SQL语句片段
type generator func(values ...interface{}) (string, []interface{})

//定义一个全局的变量，用来存储不同 SQL 语句片段生成器
var generators map[Type]generator

func init() {
	generators = make(map[Type]generator)
	generators[INSERT] = _insert
	generators[VALUES] = _values
	generators[SELECT] = _select
	generators[LIMIT] = _limit
	generators[WHERE] = _where
	generators[ORDERBY] = _orderBy
	generators[UPDATE] = _update
	generators[DELETE] = _delete
	generators[COUNT] = _count
}

//生成若干个占位符，用逗号分隔
func genBindVars(num int) string {
	vars := make([]string, num)
	for i := range vars {
		vars[i] = "?"
	}
	//将 []string 连接起来，用逗号分隔
	return strings.Join(vars, ", ")
}

// INSERT INTO $tableName ($fields)
func _insert(values ...interface{}) (string, []interface{}) {
	tableName := values[0]
	fields := strings.Join(values[1].([]string), ", ")
	return fmt.Sprintf("INSERT INTO %s (%v)", tableName, fields), []interface{}{}
}

// VALUES ($v1), ($v2), ...
func _values(values ...interface{}) (string, []interface{}) {
	var bindStr string
	var sql strings.Builder
	var vars []interface{}
	sql.WriteString("VALUES ")
	//遍历 values，将每个 value 用 genBindVars() 处理，得到 bindStr
	for i, value := range values {
		//将 value 转换为 []interface{} 类型
		v := value.([]interface{})
		//如果 bindStr 为空，则调用 genBindVars() 生成 bindStr
		if bindStr == "" {
			bindStr = genBindVars(len(v))
		}
		//将 bindStr 写入 sql，一列一个？，不同行(?,?), (?,?)
		sql.WriteString(fmt.Sprintf("(%v)", bindStr))
		//将 v 中的元素追加到 vars 中
		vars = append(vars, v...)
		//如果不是最后一个元素，则在 sql 中追加逗号
		if i+1 != len(values) {
			sql.WriteString(", ")
		}
	}
	return sql.String(), vars
}

// SELECT col1, col2, ...
//   FROM table_name
//   WHERE [ conditions ]
//   GROUP BY col1
//   HAVING [ conditions ]
func _select(values ...interface{}) (string, []interface{}) {
	// SELECT $fields FROM $tableName
	tableName := values[0]
	fields := strings.Join(values[1].([]string), ",")
	return fmt.Sprintf("SELECT %v FROM %s", fields, tableName), []interface{}{}
}

func _limit(values ...interface{}) (string, []interface{}) {
	// LIMIT $num
	return "LIMIT ?", values
}

func _where(values ...interface{}) (string, []interface{}) {
	// WHERE $desc
	desc, vars := values[0], values[1:]
	return fmt.Sprintf("WHERE %s", desc), vars
}

func _orderBy(values ...interface{}) (string, []interface{}) {
	return fmt.Sprintf("ORDER BY %s", values[0]), []interface{}{}
}

func _update(values ...interface{}) (string, []interface{}) {
	tableName, m := values[0], values[1].(map[string]interface{})
	var keys []string
	var vars []interface{}
	for key, value := range m {
		keys = append(keys, key+" = ?")
		vars = append(vars, value)
	}
	return fmt.Sprintf("UPDATE %s SET %s", tableName, strings.Join(keys, ", ")), vars
}

func _delete(values ...interface{}) (string, []interface{}) {
	return fmt.Sprintf("DELETE FROM %s", values[0]), []interface{}{}
}

func _count(values ...interface{}) (string, []interface{}) {
	return _select(values[0], []string{"count(*)"})
}

package geeorm

import (
	"database/sql"
	"fmt"
	"geeorm/dialect"
	"geeorm/log"
	"geeorm/session"
	"strings"
)

type Engine struct {
	db      *sql.DB
	dialect dialect.Dialect
}

type TxFunc func(*session.Session) (interface{}, error)

func NewEngine(driver, source string) (e *Engine, err error) {
	// 通过驱动名和数据库名打开一个数据库，得到一个 *sql.DB 对象
	db, err := sql.Open(driver, source)
	if err != nil {
		log.Error(err)
		return
	}
	// Send a ping to make sure the database connection is alive.
	if err = db.Ping(); err != nil {
		log.Error(err)
		return
	}
	// make sure the specific dialect exists
	dial, ok := dialect.GetDialect(driver)
	if !ok {
		log.Errorf("dialect %s Not Found", driver)
		return
	}

	e = &Engine{
		db:      db,
		dialect: dial,
	}
	log.Info("Connect database success")
	return
}

func (engine *Engine) Close() {
	if err := engine.db.Close(); err != nil {
		log.Error("Failed to close database")
	}
	log.Info("Close database success")
}

func (engine *Engine) NewSession() *session.Session {
	return session.New(engine.db, engine.dialect)
}

// 传入函数 f，将其作为参数传递给 TxFunc，执行 f 函数，返回结果
// 实现单行事务
func (engine *Engine) Transaction(f TxFunc) (result interface{}, err error) {
	// 创建 Session 对象
	s := engine.NewSession()
	// 开启事务
	if err = s.Begin(); err != nil {
		return nil, err
	}
	// 执行 f 函数
	defer func() {
		// 如果发生错误，回滚事务
		if p := recover(); p != nil {
			_ = s.Rollback()
			panic(p)
		} else if err != nil {
			// 如果发生错误，回滚事务
			_ = s.Rollback()
		} else {
			// 否则提交事务
			err = s.Commit()
		}
	}()
	// 执行 f 函数
	return f(s)
}

func difference(a, b []string) (diff []string) {
	mapB := make(map[string]bool)
	for _, v := range b {
		mapB[v] = true
	}
	// 遍历 a，如果 b 中不存在 a 的元素，将其加入 diff
	for _, v := range a {
		if _, ok := mapB[v]; !ok {
			diff = append(diff, v)
		}
	}
	return
}

func (engine *Engine) Migrate(value interface{}) error {
	_, err := engine.Transaction(func(s *session.Session) (result interface{}, err error) {
		// 如果表不存在，创建表
		if !s.Model(value).HasTable() {
			log.Infof("table %s doesn't exist", s.RefTable().Name)
			return nil, s.CreateTable()
		}
		// 如果表存在，比较字段
		table := s.RefTable()
		// 获取表中所有的字段名
		rows, _ := s.Raw(fmt.Sprintf("SELECT * FROM %s LIMIT 1", table.Name)).QueryRows()
		columns, _ := rows.Columns()
		//新表 - 旧表 = 新增字段
		addCols := difference(table.FieldNames, columns)
		//旧表 - 新表 = 删除字段
		delCols := difference(columns, table.FieldNames)
		log.Infof("added cols %v, deleted cols %v", addCols, delCols)

		// 删除多余的字段
		for _, col := range addCols {
			f := table.GetField(col)
			// 构造 ADD 语句
			sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", table.Name, f.Name, f.Type)
			if _, err = s.Raw(sql).Exec(); err != nil {
				return
			}
		}

		if len(delCols) == 0 {
			return
		}
		/*
			CREATE TABLE new_table AS SELECT col1, col2, ... from old_table
			DROP TABLE old_table
			ALTER TABLE new_table RENAME TO old_table;
		*/
		// 生成临时表名
		tmp := "tmp_" + table.Name
		// 重命名原始表
		fieldStr := strings.Join(table.FieldNames, ",")
		s.Raw(fmt.Sprintf("CREATE TABLE %s AS SELECT %s from %s;", tmp, fieldStr, table.Name))
		// 删除原始表
		s.Raw(fmt.Sprintf("DROP TABLE %s;", table.Name))
		// 重新创建原始表
		s.Raw(fmt.Sprintf("ALTER TABLE %s RENAME TO %s;", tmp, table.Name))
		_, err = s.Exec()
		return
	})
	return err
}

//封装事务操作
package session

import "geeorm/log"

func (s *Session) Begin() (err error) {
	log.Info("transaction begin")
	//开启事务
	if s.tx, err = s.db.Begin(); err != nil {
		log.Error(err)
		return
	}
	return
}

func (s *Session) Commit() (err error) {
	log.Info("transaction commit")
	//提交事务
	if err = s.tx.Commit(); err != nil {
		log.Error(err)
		return
	}
	return
}

func (s *Session) Rollback() (err error) {
	log.Info("transaction rollback")
	//回滚事务
	if err = s.tx.Rollback(); err != nil {
		log.Error(err)
		return
	}
	return
}

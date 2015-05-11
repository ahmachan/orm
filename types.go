// Copyright 2015 by caixw, All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package orm

import (
	"io"
)

type Dialect interface {
	Quote(w io.Writer, colName string) error

	// 从dataSourceName变量中获取数据库的名称。
	GetDBName(dataSourceName string) string

	// 生成LIMIT N OFFSET M 或是相同的语意的语句。
	// offset值为一个可选参数，若不指定，则表示LIMIT N语句。
	// 返回的是对应数据库的limit语句以及语句中占位符对应的值。
	LimitSQL(limit interface{}, offset ...interface{}) (sql string)

	// 根据数据模型，创建表。
	CreateTableSQL(m *Model) (sql string, err error)

	// 清空表内容，重置AI。
	TruncateTableSQL(tableName string) (sql string)
}

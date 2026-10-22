// Package migrations仅嵌入问答DDL供启动保护核验，不在运行时自动执行迁移。
package migrations

import _ "embed"

//go:embed 20261021_create_tm_ai_qa.sql
var QASchema string

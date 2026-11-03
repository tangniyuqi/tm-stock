package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	mysql "github.com/go-sql-driver/mysql"
)

type AIDossierManagementConfig struct {
	Actors      []int64
	PrepareDSN  string
	Corrections bool
}

func (AIDossierManagementConfig) String() string {
	return "专题管理装配（凭据与角色已隐藏）"
}
func (c AIDossierManagementConfig) GoString() string { return c.String() }

// 只在原审核凭据中显式选出的操作人能取得专题发布/读取许可管理能力。
func LoadAIDossierManagement(reviewers []AIAdminReviewer, dsn string) (*AIDossierManagementConfig, error) {
	on, err := feedbackFlag("TM_AI_DOSSIER_MANAGEMENT_ADMIN_ENABLED")
	if err != nil {
		return nil, err
	}
	raw := os.Getenv("TM_AI_DOSSIER_MANAGEMENT_ACTORS")
	prepare := os.Getenv("TM_AI_DOSSIER_MANAGEMENT_PREPARE_DSN")
	corrections, err := feedbackFlag("TM_AI_DOSSIER_CORRECTIONS_ADMIN_ENABLED")
	if err != nil {
		return nil, err
	}
	if !on {
		if raw != "" || prepare != "" || corrections {
			return nil, fmt.Errorf("专题管理装配只能与显式管理开关共同配置")
		}
		return nil, nil
	}
	if raw == "" {
		return nil, fmt.Errorf("专题管理必须显式指定原审核凭据中的操作人")
	}
	known := map[int64]bool{}
	for _, r := range reviewers {
		known[r.ActorID] = true
	}
	ids := []int64{}
	seen := map[int64]bool{}
	for _, s := range strings.Split(raw, ",") {
		id, e := strconv.ParseInt(s, 10, 64)
		if e != nil || id <= 0 || id > 9007199254740991 || strconv.FormatInt(id, 10) != s || !known[id] || seen[id] {
			return nil, fmt.Errorf("专题管理操作人范围无效")
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) > 100 {
		return nil, fmt.Errorf("专题管理操作人范围过大")
	}
	current, e1 := mysql.ParseDSN(dsn)
	owner, e2 := mysql.ParseDSN(prepare)
	if prepare == "" || e1 != nil || e2 != nil || current.Net != "tcp" || owner.Net != "tcp" || current.Addr != owner.Addr || current.DBName == "" || current.DBName != owner.DBName || current.User == "" || owner.User == "" || current.User == owner.User {
		return nil, fmt.Errorf("专题管理结构准备连接须显式指向同一数据库实例")
	}
	return &AIDossierManagementConfig{Actors: ids, PrepareDSN: prepare, Corrections: corrections}, nil
}

func LoadAILocalDossierCorrections(dossiersEnabled bool) (bool, error) {
	on, err := feedbackFlag("TM_AI_LOCAL_DOSSIER_CORRECTIONS_ENABLED")
	if err != nil {
		return false, err
	}
	if on && !dossiersEnabled {
		return false, fmt.Errorf("更正声明读取需要已有专题读取入口")
	}
	return on, nil
}

package member

import "github.com/flipped-aurora/gin-vue-admin/server/global"

// MemberAssetLog 会员资产流水（对应 addon_member_asset_log 表）
// 嵌入 GVA_MODEL_ADDON 提供标准审计列；该表已在 initialize/gorm_biz.go 的
// AutoMigrate 中注册，启动时会自动补齐缺失列（deleted_at 等）。
type MemberAssetLog struct {
	global.GVA_MODEL_ADDON
	MemberId  uint    `json:"member_id" gorm:"column:member_id;index;comment:用户ID"`
	Type      uint8   `json:"type"  gorm:"column:type;comment:类型(1余额/2积分/3虚拟币/4成长值/5经验值)"`
	Flow      uint8   `json:"flow"  gorm:"column:flow;comment:流向(1增加/2减少)"`
	BizGroup  string  `json:"biz_group" gorm:"column:biz_group;size:30;comment:业务大类"`
	BizType   string  `json:"biz_type" gorm:"column:biz_type;size:30;comment:关联业务类型"`
	BizId     uint    `json:"biz_id" gorm:"column:biz_id;comment:关联业务ID"`
	BeforeNum float64 `json:"before_num" gorm:"column:before_num;type:decimal(12,2);comment:变动前数值"`
	AfterNum  float64 `json:"after_num" gorm:"column:after_num;type:decimal(12,2);comment:变动后数值"`
	ChangeNum float64 `json:"change_num" gorm:"column:change_num;type:decimal(12,2);comment:变动数值"`
	Remark    string  `json:"remark" gorm:"column:remark;size:200;comment:备注信息"`
	IP        string  `json:"ip" gorm:"column:ip;size:45;comment:IP"`
}

// TableName 会员资产流水表 addon_member_asset_log
func (MemberAssetLog) TableName() string {
	return "addon_member_asset_log"
}

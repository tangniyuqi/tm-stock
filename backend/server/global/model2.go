package global

import (
	"time"

	"gorm.io/gorm"
)

type GVA_MODEL_ADDON struct {
	ID        uint           `gorm:"primarykey;column:id;comment:ID" json:"id"`        // 主键ID
	CreatedAt time.Time      `gorm:"column:created_at;comment:创建时间" json:"created_at"` // 创建时间
	UpdatedAt time.Time      `gorm:"column:updated_at;comment:更新时间" json:"updated_at"` // 更新时间
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at;comment:删除时间" json:"deleted_at"`    // 删除时间
	CreatedBy uint           `gorm:"column:created_by;comment:创建者" json:"created_by"`  // 创建者用户ID
	UpdatedBy uint           `gorm:"column:updated_by;comment:更新者" json:"updated_by"`  // 最近更新者用户ID
	DeletedBy uint           `gorm:"column:deleted_by;comment:删除者" json:"deleted_by"`  // 删除者用户ID
}

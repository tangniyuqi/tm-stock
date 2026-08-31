// 自动生成模板Theme
package quant

import (
	"fmt"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"gorm.io/gorm"
)

// 题材 结构体  Theme
type Theme struct {
	global.GVA_MODEL_ADDON
	Name        *string  `json:"name" form:"name" gorm:"comment:名称;column:name;size:100;"`                                          //名称
	Code        *string  `json:"code" form:"code" gorm:"comment:代码;column:code;size:100;"`                                          //代码
	Level       *int8    `json:"level" form:"level" gorm:"default:0;comment:层级;column:level;"`                                      //层级
	ParentId    int      `json:"parent_id" form:"parent_id" gorm:"default:0;comment:父级ID;column:parent_id;"`                        //父级ID，父节点
	ChangePct   *float64 `json:"change_pct" form:"change_pct" gorm:"type:decimal(6,2);default:0.00;comment:涨跌幅;column:change_pct;"` //涨跌幅
	Description *string  `json:"description" form:"description" gorm:"comment:描述;column:description;size:250;"`                     //描述
	Remark      *string  `json:"remark" form:"remark" gorm:"comment:备注;column:remark;size:250;"`                                    //备注
	Source      *int8    `json:"source" form:"source" gorm:"default:0;comment:来源;column:source;"`                                   //来源
	Sort        *int32   `json:"sort" form:"sort" gorm:"default:0;comment:排序;column:sort;"`                                         //排序
	Status      *int8    `json:"status" form:"status" gorm:"default:1;comment:状态;column:status;"`                                   //状态
	StockCount  *int32   `json:"stock_count" form:"stock_count" gorm:"default:0;comment:股票数量;column:stock_count;"`                  //股票数量
	Children    []*Theme `json:"children" gorm:"-"` //子节点
}

// TableName 题材 Theme自定义表名 addon_quant_theme
func (Theme) TableName() string {
	return "addon_quant_theme"
}

// GetChildren 实现TreeNode接口
func (s *Theme) GetChildren() []*Theme {
	return s.Children
}

// SetChildren 实现TreeNode接口
func (s *Theme) SetChildren(children *Theme) {
	s.Children = append(s.Children, children)
}

// GetID 实现TreeNode接口
func (s *Theme) GetID() int {
	return int(s.ID)
}

// GetParentId 实现TreeNode接口
func (s *Theme) GetParentID() int {
	return s.ParentId
}

func (s *Theme) BeforeCreate(tx *gorm.DB) (err error) {
	if (s.Source == nil || *s.Source == 0) && (s.Code == nil || *s.Code == "") {
		code := fmt.Sprintf("TC%d", time.Now().Unix())
		s.Code = &code
	}
	return
}

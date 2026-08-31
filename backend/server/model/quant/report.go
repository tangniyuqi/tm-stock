// 自动生成模板Report
package quant

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"gorm.io/datatypes"
)

// 研报 结构体  Report
type Report struct {
	global.GVA_MODEL_ADDON
	Title        *string    `json:"title" form:"title" gorm:"comment:标题;column:title;size:250;" binding:"required"`      //名称
	Type         *int       `json:"type" form:"type" gorm:"default:0;comment:类型;column:type;size:1;"`                    //类型
	Industry     *string    `json:"industry" form:"industry" gorm:"comment:行业;column:industry;size:50;"`                 //行业
	Level        *int       `json:"level" form:"level" gorm:"default:0;comment:等级;column:level;size:1;"`                 //等级
	Institution  *string    `json:"institution" form:"institution" gorm:"comment:发布机构;column:institution;size:250;"`    //发布机构
	Analyst      *string    `json:"analyst" form:"analyst" gorm:"comment:分析师;column:analyst;size:250;"`                  //分析师
	PublishDate  *time.Time `json:"publish_date" form:"publish_date" gorm:"type:date;comment:发布日期;column:publish_date;"`           //发布日期
	DeadlineDate *time.Time `json:"deadline_date" form:"deadline_date" gorm:"type:date;comment:有效日期;column:deadline_date;"`       //研报有效期
	Summary      *string    `json:"summary" form:"summary" gorm:"comment:研报摘要;column:summary;"`                         //研报摘要
	FilePath     *string    `json:"file_path" form:"file_path" gorm:"comment:文件路径;column:file_path;size:250;"`          //文件路径
	FileExt      *string    `json:"file_ext" form:"file_ext" gorm:"comment:文件后缀;column:file_ext;size:10;"`              //文件后缀
	Attachments  datatypes.JSON `json:"attachments" form:"attachments" gorm:"comment:附件;column:attachments;" swaggertype:"array,object"`  //附件
	PageCount    *int       `json:"page_count" form:"page_count" gorm:"default:0;comment:页数;column:page_count;size:10;"` //页数
	Remark       *string    `json:"remark" form:"remark" gorm:"comment:备注;column:remark;size:250;"`                      //备注
	Status       *int       `json:"status" form:"status" gorm:"default:0;comment:状态;column:status;size:1;"`              //状态
}

// TableName 研报 Report自定义表名 addon_quant_report
func (Report) TableName() string {
	return "addon_quant_report"
}

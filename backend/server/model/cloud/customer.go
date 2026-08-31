// 自动生成模板Customer
package cloud

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"gorm.io/datatypes"
)

// 客户 结构体  Customer
type Customer struct {
	global.GVA_MODEL_ADDON
	MerchantId *int           `json:"merchant_id" form:"merchant_id" gorm:"default:0;comment:商户ID;column:merchant_id;type:int(10);size:10;"` //商户ID
	MemberId   *int           `json:"member_id" form:"member_id" gorm:"default:0;comment:会员ID;column:member_id;type:int(10);size:10;"`       //会员ID
	Type       *int           `json:"type" form:"type" gorm:"default:0;comment:类型;column:type;size:1;"`                                      //类型
	Name       *string        `json:"name" form:"name" gorm:"comment:姓名;column:name;size:250;" binding:"required"`                           //姓名
	Gender     *int           `json:"gender" form:"gender" gorm:"default:0;comment:性别;column:gender;size:1;"`                                //性别
	Level      *int           `json:"level" form:"level" gorm:"default:0;comment:等级;column:level;size:1;"`                                   //等级
	Tips       *string        `json:"tips" form:"tips" gorm:"comment:备注;column:tips;size:250;"`                                              //备注
	Birth      *time.Time     `json:"birth" form:"birth" gorm:"default:null;type:date;comment:出生;column:birth;"`                             //出生
	Legal      *bool          `json:"legal" form:"legal" gorm:"default:0;comment:法人;column:legal;size:1;"`                                   //法人
	Position   *string        `json:"position" form:"position" gorm:"comment:职位;column:position;size:250;"`                                  //职位
	Industry   *int           `json:"industry" form:"industry" gorm:"default:0;comment:行业;column:industry;size:3;"`                          //行业
	Company    *string        `json:"company" form:"company" gorm:"comment:公司;column:company;size:250;"`                                     //公司
	Department *string        `json:"department" form:"department" gorm:"comment:部门;column:department;size:250;"`                            //部门
	Business   *string        `json:"business" form:"business" gorm:"comment:业务;column:business;size:250;"`                                  //业务
	Phone      *string        `json:"phone" form:"phone" gorm:"comment:电话;column:phone;size:250;"`                                           //电话
	Mobile     *string        `json:"mobile" form:"mobile" gorm:"comment:手机;column:mobile;size:250;"`                                        //手机
	Wechat     *string        `json:"wechat" form:"wechat" gorm:"comment:微信;column:wechat;size:250;"`                                        //微信
	Douyin     *string        `json:"douyin" form:"douyin" gorm:"comment:抖音;column:douyin;size:250;"`                                        //抖音
	Qq         *string        `json:"qq" form:"qq" gorm:"comment:QQ;column:qq;size:250;"`                                                    //QQ
	Email      *string        `json:"email" form:"email" gorm:"comment:电子邮箱;column:email;size:250;"`                                         //电子邮箱
	Web        *string        `json:"web" form:"web" gorm:"comment:网站;column:web;size:250;"`                                                 //网站
	Source     *int           `json:"source" form:"source" gorm:"default:0;comment:来源;column:source;size:1;"`                                //来源
	LastDate   *time.Time     `json:"last_date" form:"last_date" gorm:"type:date;comment:最近联系日期;column:last_date;"`                          //最近联系日期
	NextDate   *time.Time     `json:"next_date" form:"next_date" gorm:"type:date;comment:下次联系日期;column:next_date;"`                          //下次联系日期
	ProvinceId *int           `json:"province_id" form:"province_id" gorm:"default:0;comment:省份ID;column:province_id;type:int(10);size:10;"` //省份ID
	CityId     *int           `json:"city_id" form:"city_id" gorm:"default:0;comment:城市ID;column:city_id;type:int(10);size:10;"`             //城市ID
	DistrictId *int           `json:"district_id" form:"district_id" gorm:"default:0;comment:区县ID;column:district_id;type:int(10);size:10;"` //区县ID
	Province   *string        `json:"province" form:"province" gorm:"comment:省份;column:province;size:250;"`                                  //省份
	City       *string        `json:"city" form:"city" gorm:"comment:城市;column:city;size:250;"`                                              //城市
	District   *string        `json:"district" form:"district" gorm:"comment:区县;column:district;size:250;"`                                  //区县
	Area       *string        `json:"area" form:"area" gorm:"comment:区域;column:area;size:250;"`                                              //区域
	Address    *string        `json:"address" form:"address" gorm:"comment:地址;column:address;size:250;"`                                     //地址
	Location   *string        `json:"location" form:"location" gorm:"comment:位置;column:location;size:250;"`                                  //位置
	Coords     datatypes.JSON `json:"coords" form:"coords" gorm:"comment:坐标;column:coords;" swaggertype:"object"`                            //坐标
	Remark     *string        `json:"remark" form:"remark" gorm:"comment:备注;column:remark;type:text;"`                                       //备注
	Status     *int           `json:"status" form:"status" gorm:"default:1;comment:状态;column:status;size:1;"`                                //状态
}

// TableName 客户 Customer自定义表名 addon_cloud_customer
func (Customer) TableName() string {
	return "addon_cloud_customer"
}

/* func (c *Customer) BeforeSave(tx *gorm.DB) error {
	if c.Birth == nil {
		tx.Statement.SetColumn("birth", nil)
	}

	if c.LastDate == nil {
		tx.Statement.SetColumn("last_date", nil)
	}

	if c.NextDate == nil {
		tx.Statement.SetColumn("next_date", nil)
	}

	return nil
} */

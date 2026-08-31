package addon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/addon"
	addonReq "github.com/flipped-aurora/gin-vue-admin/server/model/addon/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type WeishiApi struct{}

// CreateWeishi 创建微视账号
// @Tags Weishi
// @Summary 创建微视账号
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body addon.Weishi true "创建微视账号"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /weishi/createWeishi [post]
func (weishiApi *WeishiApi) CreateWeishi(c *gin.Context) {
	var weishi addon.Weishi
	err := c.ShouldBindJSON(&weishi)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	weishi.CreatedBy = utils.GetUserID(c)
	err = weishiService.CreateWeishi(&weishi)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteWeishi 删除微视账号
// @Tags Weishi
// @Summary 删除微视账号
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body addon.Weishi true "删除微视账号"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /weishi/deleteWeishi [delete]
func (weishiApi *WeishiApi) DeleteWeishi(c *gin.Context) {
	ID := c.Query("ID")
	userID := utils.GetUserID(c)
	err := weishiService.DeleteWeishi(ID, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteWeishiByIds 批量删除微视账号
// @Tags Weishi
// @Summary 批量删除微视账号
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /weishi/deleteWeishiByIds [delete]
func (weishiApi *WeishiApi) DeleteWeishiByIds(c *gin.Context) {
	IDs := c.QueryArray("IDs[]")
	userID := utils.GetUserID(c)
	err := weishiService.DeleteWeishiByIds(IDs, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateWeishi 更新微视账号
// @Tags Weishi
// @Summary 更新微视账号
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body addon.Weishi true "更新微视账号"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /weishi/updateWeishi [put]
func (weishiApi *WeishiApi) UpdateWeishi(c *gin.Context) {
	var weishi addon.Weishi
	err := c.ShouldBindJSON(&weishi)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	weishi.UpdatedBy = utils.GetUserID(c)
	err = weishiService.UpdateWeishi(weishi)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindWeishi 用id查询微视账号
// @Tags Weishi
// @Summary 用id查询微视账号
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用id查询微视账号"
// @Success 200 {object} response.Response{data=addon.Weishi,msg=string} "查询成功"
// @Router /weishi/findWeishi [get]
func (weishiApi *WeishiApi) FindWeishi(c *gin.Context) {
	ID := c.Query("ID")
	reweishi, err := weishiService.GetWeishi(ID)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(reweishi, c)
}

// GetWeishiList 分页获取微视账号列表
// @Tags Weishi
// @Summary 分页获取微视账号列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query addonReq.WeishiSearch true "分页获取微视账号列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /weishi/getWeishiList [get]
func (weishiApi *WeishiApi) GetWeishiList(c *gin.Context) {
	var pageInfo addonReq.WeishiSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := weishiService.GetWeishiInfoList(pageInfo, pageInfo.Order, pageInfo.Desc)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(response.PageResult{
		List:     list,
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}

// GetWeishiPublic 不需要鉴权的微视账号接口
// @Tags Weishi
// @Summary 不需要鉴权的微视账号接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /weishi/getWeishiPublic [get]
func (weishiApi *WeishiApi) GetWeishiPublic(c *gin.Context) {
	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	weishiService.GetWeishiPublic()
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的微视账号接口信息",
	}, "获取成功", c)
}

// BatchCreateWeishi 用于批量导入
// @Tags Weishi
// @Summary 用于批量导入
// @Accept application/json
// @Produce application/json
// @Param data body addon.WeishiContent true "微视内容"
// @Success 200 {object} response.Response{data=object,msg=string} "成功"
// @Router /weishi/batchCreateWeishi [POST]
func (weishiApi *WeishiApi) BatchCreateWeishi(c *gin.Context) {
	var weishiContent addon.WeishiContent
	err := c.ShouldBindJSON(&weishiContent)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	lines := strings.Split(weishiContent.Content, "\n")
	if len(lines) == 0 {
		global.GVA_LOG.Error("导入数据为空!")
		response.FailWithMessage("导入数据不能为空", c)
		return
	}

	var weishiList []addon.Weishi
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Split(line, "----")
		if len(fields) < 12 {
			global.GVA_LOG.Error("数据格式错误!", zap.String("line", line))
			response.FailWithMessage("数据格式错误!", c)
			return
		}

		// 解析认证信息
		authInfo := struct {
			IAuthType   int
			OpenID      string
			SSessionKey string
			PersonID    string
		}{}

		pairs := strings.Split(fields[2], "; ")
		for _, pair := range pairs {
			kv := strings.Split(pair, "=")
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])

			switch key {
			case "iAuthType":
				iAuthType, err := strconv.Atoi(value)
				if err != nil {
					global.GVA_LOG.Error("iAuthType 转换失败!", zap.String("value", value), zap.Error(err))
					response.FailWithMessage("iAuthType 转换失败", c)
					return
				}
				authInfo.IAuthType = iAuthType
			case "openid":
				authInfo.OpenID = value
			case "sSessionKey":
				authInfo.SSessionKey = value
			case "person_id":
				authInfo.PersonID = value
			}
		}

		if authInfo.OpenID == "" || authInfo.SSessionKey == "" || authInfo.PersonID == "" {
			global.GVA_LOG.Error("解析失败: 缺少必要的字段", zap.String("line", line))
			response.FailWithMessage("解析失败: 缺少必要的字段", c)
			return
		}

		// 将字段映射到 Weishi 结构体
		weishi := addon.Weishi{
			Account:     stringToPtr(fields[0]),
			Password:    stringToPtr(fields[1]),
			IAuthType:   intToPtr(authInfo.IAuthType),
			Main_login:  stringToPtr("qq"),
			Openid:      stringToPtr(authInfo.OpenID),
			SSessionKey: stringToPtr(authInfo.SSessionKey),
			Person_id:   stringToPtr(authInfo.PersonID),
			Nickname:    stringToPtr(fields[7]),
		}
		weishiList = append(weishiList, weishi)
	}

	err2 := weishiService.BatchCreateWeishi(weishiList)
	if err2 != nil {
		global.GVA_LOG.Error("批量创建微视账号失败!", zap.Error(err2))
		response.FailWithMessage("批量创建微视账号失败", c)
		return
	}

	response.OkWithMessage("批量导入成功!", c)
	// response.OkWithData(weishiList, c)
}

// stringToPtr 将 string 转换为 *string
func stringToPtr(s string) *string {
	return &s
}

// intToPtr 将 int 转换为 *int
func intToPtr(i int) *int {
	return &i
}

type WeishiRequest2 struct {
	Msg WeishiMsgContent `json:"msg"`
}

type WeishiRequest struct {
	Msg string `json:"msg"` // msg 是一个 JSON 字符串
}

type WeishiMsgContent struct {
	PersonID *string `json:"person_id"`
}

type WeishiCookie struct {
	IAuthType   *int    `json:"iAuthType"`
	MainLogin   *string `json:"main_login"`
	OpenID      *string `json:"openid"`
	SSessionKey *string `json:"sSessionKey"`
}

type WeishiResponse struct {
	Ret    int    `json:"ret"`
	ErrMsg string `json:"err_msg"`
}

// sendPostRequest 发送 POST 请求到远程接口，携带 body 和 cookie
func sendPostRequest(ctx context.Context, url string, body WeishiRequest, cookie WeishiCookie) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyBuffer := bytes.NewBuffer(b)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bodyBuffer)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if cookie.IAuthType != nil {
		req.AddCookie(&http.Cookie{
			Name:  "iAuthType",
			Value: fmt.Sprintf("%d", *cookie.IAuthType),
		})
	}
	if cookie.MainLogin != nil {
		req.AddCookie(&http.Cookie{
			Name:  "main_login",
			Value: *cookie.MainLogin,
		})
	}
	if cookie.OpenID != nil {
		req.AddCookie(&http.Cookie{
			Name:  "openid",
			Value: *cookie.OpenID,
		})
	}
	if cookie.SSessionKey != nil {
		req.AddCookie(&http.Cookie{
			Name:  "sSessionKey",
			Value: *cookie.SSessionKey,
		})
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// CancelWeishi 注销微视账号
// @Tags Weishi
// @Summary 注销微视账号
// @Accept application/json
// @Produce application/json
// @Param data query addonReq.WeishiSearch true "成功"
// @Success 200 {object} response.Response{data=object,msg=string} "成功"
// @Router /weishi/cancelWeishi [POST]
func (weishiApi *WeishiApi) CancelWeishi(c *gin.Context) {
	var weishi addon.Weishi
	if err := c.ShouldBindJSON(&weishi); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	url := "https://api.weishi.qq.com/trpc.weishi.weishi_h5_proxy.weishi_h5_proxy/UnRegister"

	/*body := WeishiRequest{
		Msg: WeishiMsgContent{
			PersonID: weishi.Person_id,
		},
	}*/

	msgContent := WeishiMsgContent{
		PersonID: weishi.Person_id,
	}

	msgJSON, err := json.Marshal(msgContent)

	body := WeishiRequest{
		Msg: string(msgJSON),
	}

	cookie := WeishiCookie{
		IAuthType:   weishi.IAuthType,
		MainLogin:   weishi.Main_login,
		OpenID:      weishi.Openid,
		SSessionKey: weishi.SSessionKey,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := sendPostRequest(ctx, url, body, cookie)
	if err != nil {
		global.GVA_LOG.Error("注销微视账号失败!", zap.Error(err), zap.String("url", url), zap.Any("body", body))
		response.FailWithMessage("注销微视账号失败!", c)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		global.GVA_LOG.Error("注销微视账号失败!", zap.Int("status_code", resp.StatusCode))
		response.FailWithMessage("注销微视账号失败!", c)
		return
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		global.GVA_LOG.Error("读取响应体失败!", zap.Error(err))
		response.FailWithMessage("读取响应体失败!", c)
		return
	}

	var result WeishiResponse
	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		global.GVA_LOG.Error("解析 JSON 数据出错!", zap.Error(err))
		response.FailWithMessage("解析 JSON 数据出错!", c)
		return
	}

	if result.Ret != 0 {
		response.FailWithMessage(result.ErrMsg, c)
		return
	}

	now := time.Now()
	isoString := now.UTC().Format("2006-01-02T15:04:05.999Z")
	parsedTime, err := time.Parse("2006-01-02T15:04:05.999Z", isoString)
	statusValue := 1

	weishi.UpdatedBy = utils.GetUserID(c)
	weishi.Cancelled_at = &parsedTime
	weishi.Status = &statusValue
	err = weishiService.UpdateWeishi(weishi)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败!", c)
		return
	}
	response.OkWithData(result, c)
}

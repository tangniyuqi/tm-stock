import service from '@/utils/request';

// @Tags Member
// @Summary 创建C端会员
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body member.Member true "创建会员"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"创建成功"}"
// @Router /member/member/createMember [post]
export const createMember = data => {
  return service({
    url: '/member/member/createMember',
    method: 'post',
    data,
  });
};

// @Tags Member
// @Summary 删除C端会员
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query string true "会员ID"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"删除成功"}"
// @Router /member/member/deleteMember [delete]
export const deleteMember = params => {
  return service({
    url: '/member/member/deleteMember',
    method: 'delete',
    params,
  });
};

// @Tags Member
// @Summary 批量删除C端会员
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {string} string "{"success":true,"data":{},"msg":"批量删除成功"}"
// @Router /member/member/deleteMemberByIds [delete]
export const deleteMemberByIds = params => {
  return service({
    url: '/member/member/deleteMemberByIds',
    method: 'delete',
    params,
  });
};

// @Tags Member
// @Summary 更新C端会员
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body member.Member true "更新会员"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"更新成功"}"
// @Router /member/member/updateMember [put]
export const updateMember = data => {
  return service({
    url: '/member/member/updateMember',
    method: 'put',
    data,
  });
};

// @Tags Member
// @Summary 用id查询C端会员
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query string true "会员ID"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"查询成功"}"
// @Router /member/member/findMember [get]
export const findMember = params => {
  return service({
    url: '/member/member/findMember',
    method: 'get',
    params,
  });
};

// @Tags Member
// @Summary 分页获取C端会员列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query memberReq.MemberSearch true "分页获取会员列表"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"获取成功"}"
// @Router /member/member/getMemberList [get]
export const getMemberList = params => {
  return service({
    url: '/member/member/getMemberList',
    method: 'get',
    params,
  });
};

// @Tags Member
// @Summary 后台管理 - 重置会员密码
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body member.ResetMemberPwdReq true "重置密码"
// @Success 200 {string} string "{"success":true,"data":{},"msg":"密码重置成功"}"
// @Router /member/member/resetMemberPassword [post]
export const resetMemberPassword = data => {
  return service({
    url: '/member/member/resetMemberPassword',
    method: 'post',
    data,
  });
};

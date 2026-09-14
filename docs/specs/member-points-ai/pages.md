# 页面与原型映射

核对基线：73e759b；历史原型来源 3fc1393，最近原型改动 495469b。仅恢复参考资产，不恢复旧业务代码。以下为文件与路由静态核对，不代表实际编译和接口联调通过。

## 原型资产

- docs/prototypes/theme-data-remediation/index.html：历史总览。
- p0.html：9 张核心页面画板，另有 4 张简要交互说明卡。
- p0-states.html：53 张独立状态画板，包含重复页面的加载、错误、权限等状态，不是 53 个业务页面。
- overview.png、regression-theme-detail.png：历史截图，展示时必须注明历史参考，不是本轮新设计。
- Git 作者字段不能单独证明由哪个 AI 任务或协作者制作。没有证据支持此前关于“45 张由另一位协作者制作”的推测。

## 对照清单

| 当前文件（相对仓库根目录） | 当前路由 | 参考及处理 |
|---|---|---|
| pages/index/index.uvue | 已注册 | 总览布局可参考，须适配当前动态接口与会员入口 |
| pages/segment/index.uvue | 已注册 | P0 题材库；复用搜索、空态、错误态 |
| pages/segment/detail.uvue | 已注册 | P0 详情及历史 PNG；保留环节折叠和归属依据，替换旧试吃权限 |
| pages/segment/components/ThemeEvidenceSheet.uvue | 内嵌组件 | 复用依据、来源与外链确认状态，不作为独立业务路由 |
| pages/news/index.uvue、detail.uvue | 已注册 | 总览资料流可参考；保留来源、详情失败与返回状态 |
| pages/ai/index.uvue | 已注册 | 当前为敬请期待；历史资料整理页不覆盖新 AI，需新原型 |
| pages/member/index.uvue | 已注册 | P0 会员布局可复用，须新增付费开通、初始积分及贡献入口 |
| pages/member/login.uvue | 已注册 | P0 登录错误反馈可借鉴；现代码与旧验证码流程不能直接视为一致 |
| pages/member/redeem.uvue | 已注册 | 旧兑换码入口不匹配现商业路径，不作为开通会员主流程 |
| pages/member/privacy.uvue、service.uvue | 已注册 | 法律与服务说明布局可复用，旧无支付文本需重新审定 |
| pages/theme/index.uvue、detail.uvue、treasure.uvue | 未注册 | 与 segment 路径并存，需核对接口职责与入口，不能简单批量挂路由 |
| pages/theme_topic/detail.uvue | 未注册 | 用户截图是新动态/逻辑详情的重要参考，补积分预览、解锁及投稿状态 |
| pages/member/asset/index.uvue | 未注册 | 已有资产页面，会员页也有跳转；补充值、冻结与贡献账目原型 |
| pages/member/register.uvue、forget.uvue、reset-password.uvue | 未注册 | 注册/找回流程需与当前身份接口核对，旧原型覆盖不完整 |
| pages/member/setting 下 index、account、change-password、mobile、profile、realname | 未注册 | 会员页已有设置跳转；页面存在不代表可访问，实名功能需先确认必要性 |
| pages/cms 下 feedback/index、help/index、page/detail | 未注册 | 会员页已有跳转，需补路由及错误返回验证；原型覆盖不完整 |

当前 pages.json 共注册 11 个页面。PageHeader、首页 table 与依据弹层为组件，不计业务页面数量。原型中的交互展示不能替代真实客户端验收。

## 按顺序实施

1. 完成本清单与历史资产恢复；不改变业务页面。
2. 制作第一组新图：付费会员基础题材页、逻辑详情付费预览、已解锁详情、补充逻辑。采用已有红白配色、卡片和底部弹层；积分价格待定时不编造正式价格。
3. 用户确认后，明确 segment/theme/theme_topic 各入口职责及 API 契约，再补路由、权限与页面；验证已登录、未登录、无会员、余额不足、已购买、退款和内容下线。
4. 制作并确认会员开通、积分充值/流水、贡献审核与奖励、AI 任务与结果图，再逐项开发。
5. 广告位按证券开户/银行卡开户分别设计，标明广告、服务主体、地区和外链；无真实合作素材时保持未投放状态。

## 本项核验与限制

恢复的三个 HTML 入口保留历史标识；所有业务页面和 pages.json 均未修改。现有题材详情 PNG 已目视核验，含明显示例标识。浏览器 URL 安全策略拒绝打开本地 HTML，未绕过，故本轮不声称浏览器渲染通过。

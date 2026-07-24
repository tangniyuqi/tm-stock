# tmx-ui 组件完整目录

> **检索规则（AI 必须遵循）：**
> 1. 编写页面或组件时，使用任何 `<x-xxx>` 组件前，**必须先在下表中找到对应的文档路径**
> 2. **读取该文档**了解完整 Props、Events、Slots、Ref 方法和示例用法
> 3. 如果不确定用哪个组件，先浏览本目录按分类查找
> 4. 组件均为 easycom 自动注册，直接在 template 中使用，无需手动 import

---

## 常用基础

| 组件 | 说明 | 文档 |
|------|------|------|
| `x-sheet` | 容器/卡片。**禁止多层嵌套** | [tmx-ui/常用组件/容器 xSheet.md](tmx-ui/常用组件/容器%20xSheet.md) |
| `x-button` | 按钮 | [tmx-ui/常用组件/按钮 xButton.md](tmx-ui/常用组件/按钮%20xButton.md) |
| `x-text` | 文本 | [tmx-ui/常用组件/文字 xText.md](tmx-ui/常用组件/文字%20xText.md) |
| `x-icon` | 图标 | [tmx-ui/常用组件/图标 xIcon.md](tmx-ui/常用组件/图标%20xIcon.md) |
| `x-tag` | 标签 | [tmx-ui/常用组件/标签 xTag.md](tmx-ui/常用组件/标签%20xTag.md) |
| `x-image` | 图片 | [tmx-ui/展示组件/图片 xImage.md](tmx-ui/展示组件/图片%20xImage.md) |
| `x-link` | 链接 | [tmx-ui/展示组件/链接 xLink.md](tmx-ui/展示组件/链接%20xLink.md) |
| `x-divider` | 分割线 | [tmx-ui/常用组件/分割线 xDivider.md](tmx-ui/常用组件/分割线%20xDivider.md) |
| `x-loading` | 加载 | [tmx-ui/反馈组件/加载中 xLoading.md](tmx-ui/反馈组件/加载中%20xLoading.md) |
| `x-empty` | 空状态 | [tmx-ui/常用组件/空页面 xEmpty.md](tmx-ui/常用组件/空页面%20xEmpty.md) |
| `x-badge` | 徽标 | [tmx-ui/常用组件/徽标 xBadge.md](tmx-ui/常用组件/徽标%20xBadge.md) |
| `x-money` | 金额 | [tmx-ui/展示组件/金额栅格 xMoney.md](tmx-ui/展示组件/金额栅格%20xMoney.md) |

---

## 布局

> **重要：tmx-ui 容器类组件（`x-sheet`、`x-row`、`x-col` 等）禁止二次嵌套。**
> 外层套一层 tmx-ui 容器后，内部应使用 `view`、`scroll-view` 等官方基础组件进行布局。

| 组件 | 说明 | 文档 |
|------|------|------|
| `x-row` | 栅格行容器，**不可嵌套** | [tmx-ui/常用组件/布局 xRow.md](tmx-ui/常用组件/布局%20xRow.md) |
| `x-col` | 栅格列 | [tmx-ui/常用组件/布局子组件 xCol.md](tmx-ui/常用组件/布局子组件%20xCol.md) |
| `x-layout` | 占位排版 | [tmx-ui/其它组件/占位排版 xLayout.md](tmx-ui/其它组件/占位排版%20xLayout.md) |
| `x-sticky` | 粘性吸附 | [tmx-ui/导航组件/粘性布局 Sticky.md](tmx-ui/导航组件/粘性布局%20Sticky.md) |
| `x-scrollx` | 横向滚动 | [tmx-ui/常用组件/横向滚动 xScrollx.md](tmx-ui/常用组件/横向滚动%20xScrollx.md) |
| `x-more` | 查看更多 | [tmx-ui/展示组件/查看更多 xMore.md](tmx-ui/展示组件/查看更多%20xMore.md) |
| `x-view-tofull` | 点击展开全屏 | [tmx-ui/其它组件/全屏展开 xViewTofull.md](tmx-ui/其它组件/全屏展开%20xViewTofull.md) |

---

## 导航

| 组件 | 说明 | 文档 |
|------|------|------|
| `x-navbar` | 顶部导航栏 | [tmx-ui/导航组件/标题导航 xNavbar.md](tmx-ui/导航组件/标题导航%20xNavbar.md) |
| `x-tabs` | 标签页 | [tmx-ui/导航组件/标签导航 xTabs.md](tmx-ui/导航组件/标签导航%20xTabs.md) |
| `x-tabbar` | 底部导航栏 | [tmx-ui/导航组件/底部导航 xTabbar.md](tmx-ui/导航组件/底部导航%20xTabbar.md) |
| `x-grid` + `x-grid-item` | 宫格导航 | [tmx-ui/导航组件/宫格 xGrid.md](tmx-ui/导航组件/宫格%20xGrid.md) |
| `x-pagination` | 分页器 | [tmx-ui/表单组件/翻页器 xPagination.md](tmx-ui/表单组件/翻页器%20xPagination.md) |
| `x-search` | 搜索栏 | [tmx-ui/导航组件/搜索栏 xSearch.md](tmx-ui/导航组件/搜索栏%20xSearch.md) |
| `x-backtop` | 返回顶部 | [tmx-ui/导航组件/返回顶部 xBacktop.md](tmx-ui/导航组件/返回顶部%20xBacktop.md) |
| `x-indexbar` | 索引列表 | [tmx-ui/导航组件/索引 xIndexbar.md](tmx-ui/导航组件/索引%20xIndexbar.md) |
| `x-steps` + `x-steps-item` | 步骤条 | [tmx-ui/展示组件/步骤条 xSteps.md](tmx-ui/展示组件/步骤条%20xSteps.md) |
| `x-weekbar` | 周选择条 | [tmx-ui/导航组件/周选择 xWeekbar.md](tmx-ui/导航组件/周选择%20xWeekbar.md) |

---

## 表单

| 组件 | 说明 | 文档 |
|------|------|------|
| `x-form` + `x-form-item` | 表单验证容器 | [tmx-ui/表单组件/表单 xForm.md](tmx-ui/表单组件/表单%20xForm.md) |
| `x-input` | 输入框 | [tmx-ui/表单组件/输入框 xInput.md](tmx-ui/表单组件/输入框%20xInput.md) |
| `x-input-number` | 数字输入框 | [tmx-ui/表单组件/数字输入框 xInputNumber.md](tmx-ui/表单组件/数字输入框%20xInputNumber.md) |
| `x-input-tag` | 标签输入框 | [tmx-ui/表单组件/标签输入框 xInputTag.md](tmx-ui/表单组件/标签输入框%20xInputTag.md) |
| `x-checkbox` + `x-checkbox-group` | 多选框 | [tmx-ui/表单组件/多选框 xCheckbox.md](tmx-ui/表单组件/多选框%20xCheckbox.md) |
| `x-radio` + `x-radio-group` | 单选框 | [tmx-ui/表单组件/单选框 xRadio.md](tmx-ui/表单组件/单选框%20xRadio.md) |
| `x-radio-button` | 单选按钮组 | [tmx-ui/表单组件/单选按钮组 xRadioButton.md](tmx-ui/表单组件/单选按钮组%20xRadioButton.md) |
| `x-switch` | 开关 | [tmx-ui/表单组件/开关 xSwitch.md](tmx-ui/表单组件/开关%20xSwitch.md) |
| `x-slider` | 单向滑块 | [tmx-ui/表单组件/单向滑块 xSlider.md](tmx-ui/表单组件/单向滑块%20xSlider.md) |
| `x-slider-double` | 双向范围滑块 | [tmx-ui/表单组件/双向滑块 xSliderDouble.md](tmx-ui/表单组件/双向滑块%20xSliderDouble.md) |
| `x-stepper` | 步进器 | [tmx-ui/表单组件/步进器 xStepper.md](tmx-ui/表单组件/步进器%20xStepper.md) |
| `x-rate` | 评分 | [tmx-ui/表单组件/评分 xRate.md](tmx-ui/表单组件/评分%20xRate.md) |
| `x-picker` | 级联选择器 | [tmx-ui/表单组件/选择器 xPicker.md](tmx-ui/表单组件/选择器%20xPicker.md) |
| `x-picker-view` | 内嵌选择器 | [tmx-ui/表单组件/选择器容器 xPickerView.md](tmx-ui/表单组件/选择器容器%20xPickerView.md) |
| `x-picker-date` | 日期选择器 | [tmx-ui/表单组件/日期选择器 xPickerDate.md](tmx-ui/表单组件/日期选择器%20xPickerDate.md) |
| `x-picker-time` | 时间选择器 | [tmx-ui/表单组件/时间选择器 xPickerTime.md](tmx-ui/表单组件/时间选择器%20xPickerTime.md) |
| `x-picker-city` | 城市选择器 | [tmx-ui/表单组件/城市选择器 xPickerCity.md](tmx-ui/表单组件/城市选择器%20xPickerCity.md) |
| `x-picker-selected` | 搜索选择器 | [tmx-ui/表单组件/搜索选择器 xPickerSelected.md](tmx-ui/表单组件/搜索选择器%20xPickerSelected.md) |
| `x-between-time` | 时间区间 | [tmx-ui/表单组件/时间区间选择 xBetweenTime.md](tmx-ui/表单组件/时间区间选择%20xBetweenTime.md) |
| `x-calendar-view` | 日历视图 | [tmx-ui/表单组件/日历视图 xCalendarView.md](tmx-ui/表单组件/日历视图%20xCalendarView.md) |
| `x-calendar-multiple` | 日历多选 | [tmx-ui/表单组件/多选日历 xCalendarMultiple.md](tmx-ui/表单组件/多选日历%20xCalendarMultiple.md) |
| `x-date-view` | 日期视图 | [tmx-ui/表单组件/嵌入式日期选择器 xDateView.md](tmx-ui/表单组件/嵌入式日期选择器%20xDateView.md) |
| `x-upload-media` | 图片/视频上传 | [tmx-ui/表单组件/图片上传 xUploadMedia.md](tmx-ui/表单组件/图片上传%20xUploadMedia.md) |
| `x-upload-file` | 文件上传 | [tmx-ui/表单组件/文件选择 xUploadFile.md](tmx-ui/表单组件/文件选择%20xUploadFile.md) |
| `x-code-input` | 验证码输入框 | [tmx-ui/表单组件/验证码输入 xCodeInput.md](tmx-ui/表单组件/验证码输入%20xCodeInput.md) |
| `x-color-view` | 颜色选择器 | [tmx-ui/其它组件/颜色选择 xColorView.md](tmx-ui/其它组件/颜色选择%20xColorView.md) |
| `x-sign-board` | 签名板 | [tmx-ui/其它组件/签名板 xSignBoard.md](tmx-ui/其它组件/签名板%20xSignBoard.md) |
| `x-slide-verify` | 滑动验证 | [tmx-ui/其它组件/滑动验证 xSlideVerify.md](tmx-ui/其它组件/滑动验证%20xSlideVerify.md) |
| `x-mention` | @提及 | [tmx-ui/其它组件/提及 xMention.md](tmx-ui/其它组件/提及%20xMention.md) |

---

## 展示

| 组件 | 说明 | 文档 |
|------|------|------|
| `x-cell` | 单元格/列表项 | [tmx-ui/展示组件/列表 xCell.md](tmx-ui/展示组件/列表%20xCell.md) |
| `x-card` | 卡片 | [tmx-ui/展示组件/卡片 xCard.md](tmx-ui/展示组件/卡片%20xCard.md) |
| `x-collapse` + `x-collapse-item` | 折叠面板 | [tmx-ui/展示组件/折叠面板 xCollapse.md](tmx-ui/展示组件/折叠面板%20xCollapse.md) |
| `x-progress` | 线性进度条 | [tmx-ui/展示组件/进度条 xProgress.md](tmx-ui/展示组件/进度条%20xProgress.md) |
| `x-circle-progress` | 圆形进度环 | [tmx-ui/展示组件/圆形进度环 xCircleProgress.md](tmx-ui/展示组件/圆形进度环%20xCircleProgress.md) |
| `x-notice` | 通知栏 | [tmx-ui/展示组件/通知栏 xNotice.md](tmx-ui/展示组件/通知栏%20xNotice.md) |
| `x-msg-notice` | 消息通知条 | [tmx-ui/展示组件/消息通知 xMsgNotice.md](tmx-ui/展示组件/消息通知%20xMsgNotice.md) |
| `x-swiper` + `x-swiper-item` | 轮播 | [tmx-ui/展示组件/轮播 xSwiper.md](tmx-ui/展示组件/轮播%20xSwiper.md) |
| `x-swiper-c` | 动效轮播 | [tmx-ui/展示组件/动效轮播 xSwiperC.md](tmx-ui/展示组件/动效轮播%20xSwiperC.md) |
| `x-avatar-group` | 头像组 | [tmx-ui/展示组件/头像组 xAvatarGroup.md](tmx-ui/展示组件/头像组%20xAvatarGroup.md) |
| `x-image-group` | 图集 | [tmx-ui/展示组件/图集 xImageGroup.md](tmx-ui/展示组件/图集%20xImageGroup.md) |
| `x-image-resizer` | 图片裁剪 | [tmx-ui/其它组件/图片裁剪 xImageResizer.md](tmx-ui/其它组件/图片裁剪%20xImageResizer.md) |
| `x-skeleton` | 骨架屏 | [tmx-ui/展示组件/骨架屏 xSkeleton.md](tmx-ui/展示组件/骨架屏%20xSkeleton.md) |
| `x-countdown` | 倒计时 | [tmx-ui/展示组件/倒计时 xCountdown.md](tmx-ui/展示组件/倒计时%20xCountdown.md) |
| `x-rolling-number` | 数字翻滚 | [tmx-ui/展示组件/数字翻滚 xRollingNumber.md](tmx-ui/展示组件/数字翻滚%20xRollingNumber.md) |
| `x-barcode` | 条码 | [tmx-ui/其它组件/条码 xBarcode.md](tmx-ui/其它组件/条码%20xBarcode.md) |
| `x-qrcoder` | 二维码 | [tmx-ui/其它组件/二维码 xQrcoder.md](tmx-ui/其它组件/二维码%20xQrcoder.md) |
| `x-watermark` | 水印 | [tmx-ui/展示组件/水印 xWatermark.md](tmx-ui/展示组件/水印%20xWatermark.md) |
| `x-text-cloud` | 词云 | [tmx-ui/展示组件/词云 TextCloud.md](tmx-ui/展示组件/词云%20TextCloud.md) |
| `x-cmarkdown` | Markdown 渲染 | [tmx-ui/展示组件/markdown xCmarkdown.md](tmx-ui/展示组件/markdown%20xCmarkdown.md) |
| `x-table` | 表格 | [tmx-ui/展示组件/表格 xTable.md](tmx-ui/展示组件/表格%20xTable.md) |
| `x-tree` | 树形结构 | [tmx-ui/展示组件/树形 xTree.md](tmx-ui/展示组件/树形%20xTree.md) |
| `x-tree-flat` | 思维导图 | [tmx-ui/其它组件/思维导图 xTreeFlat.md](tmx-ui/其它组件/思维导图%20xTreeFlat.md) |
| `x-virtual-list` | 虚拟列表 | [tmx-ui/展示组件/虚拟列表 xVirtualList.md](tmx-ui/展示组件/虚拟列表%20xVirtualList.md) |
| `x-waterfall` + `x-waterfall-item` | 瀑布流 | [tmx-ui/展示组件/瀑布流 xWaterfall.md](tmx-ui/展示组件/瀑布流%20xWaterfall.md) |
| `x-echart` | ECharts 图表 | [tmx-ui/其它组件/图表 xEchart.md](tmx-ui/其它组件/图表%20xEchart.md) |
| `x-editor` | 富文本编辑器 | [tmx-ui/表单组件/富文本编辑器 xEditor.md](tmx-ui/表单组件/富文本编辑器%20xEditor.md) |
| `x-alert` | 警告提示 | [tmx-ui/展示组件/警告 xAlert.md](tmx-ui/展示组件/警告%20xAlert.md) |
| `x-barrage` | 弹幕 | [tmx-ui/反馈组件/弹幕 Barrage.md](tmx-ui/反馈组件/弹幕%20Barrage.md) |

---

## 反馈交互

| 组件 | 说明 | 文档 |
|------|------|------|
| `x-modal` | 模态弹窗 | [tmx-ui/反馈组件/弹窗 xModal.md](tmx-ui/反馈组件/弹窗%20xModal.md) |
| `x-action-modal` | 操作确认弹窗 | [tmx-ui/反馈组件/动作弹窗 xActionModal.md](tmx-ui/反馈组件/动作弹窗%20xActionModal.md) |
| `x-drawer` | 抽屉 | [tmx-ui/反馈组件/抽屉 xDrawer.md](tmx-ui/反馈组件/抽屉%20xDrawer.md) |
| `x-float-drawer` | 浮动抽屉 | [tmx-ui/反馈组件/浮动面板 FloatDrawer.md](tmx-ui/反馈组件/浮动面板%20FloatDrawer.md) |
| `x-action-menu` | 操作菜单 | [tmx-ui/反馈组件/动作菜单面板 xActionMenu.md](tmx-ui/反馈组件/动作菜单面板%20xActionMenu.md) |
| `x-overlay` | 遮罩层 | [tmx-ui/反馈组件/遮罩 xOverlay.md](tmx-ui/反馈组件/遮罩%20xOverlay.md) |
| `x-dropdown-menu` + `x-dropdown-item` | 下拉筛选 | [tmx-ui/反馈组件/下拉菜单 xDropdownMenu.md](tmx-ui/反馈组件/下拉菜单%20xDropdownMenu.md) |
| `x-popover` | 气泡菜单 | [tmx-ui/反馈组件/气泡菜单 xPopover.md](tmx-ui/反馈组件/气泡菜单%20xPopover.md) |
| `x-float-button` | 浮球按钮 | [tmx-ui/反馈组件/浮动按钮 xFloatButton.md](tmx-ui/反馈组件/浮动按钮%20xFloatButton.md) |
| `x-snackbar` | 消息条 | [tmx-ui/反馈组件/消息条 xSnackbar.md](tmx-ui/反馈组件/消息条%20xSnackbar.md) |
| `x-pull-refresh` | 下拉刷新 | [tmx-ui/反馈组件/下拉刷新 xPullRefresh.md](tmx-ui/反馈组件/下拉刷新%20xPullRefresh.md) |
| `x-slider-menu` | 侧边抽屉菜单 | [tmx-ui/导航组件/侧边菜单 xSliderMenu.md](tmx-ui/导航组件/侧边菜单%20xSliderMenu.md) |
| `x-switch-slider` | 左滑操作菜单 | [tmx-ui/反馈组件/左滑菜单 xSwitchSlider.md](tmx-ui/反馈组件/左滑菜单%20xSwitchSlider.md) |
| `x-slider-tree` | 侧边分类树 | [tmx-ui/导航组件/侧边分类 xSliderTree.md](tmx-ui/导航组件/侧边分类%20xSliderTree.md) |

---

## 手势与动画

| 组件 | 说明 | 文档 |
|------|------|------|
| `x-animation` | 动画组件 | [tmx-ui/其它组件/动画 xAnimation.md](tmx-ui/其它组件/动画%20xAnimation.md) |
| `x-finger` | 手势识别 | [tmx-ui/其它组件/手势 xFinger.md](tmx-ui/其它组件/手势%20xFinger.md) |
| `x-drag` + `x-drag-item` | 拖拽排序 | [tmx-ui/反馈组件/拖拽排序 xDrag.md](tmx-ui/反馈组件/拖拽排序%20xDrag.md) |

---

## 键盘

| 组件 | 说明 | 文档 |
|------|------|------|
| `x-keyboard` | 密码键盘 | [tmx-ui/表单组件/密码键盘 xKeyboard.md](tmx-ui/表单组件/密码键盘%20xKeyboard.md) |
| `x-keyboard-number` | 纯数字键盘 | [tmx-ui/表单组件/数字键盘 xKeyboardNumber.md](tmx-ui/表单组件/数字键盘%20xKeyboardNumber.md) |
| `x-keyboard-car` | 车牌键盘 | [tmx-ui/表单组件/车牌键盘 xKeyboardCar.md](tmx-ui/表单组件/车牌键盘%20xKeyboardCar.md) |
| `x-keyboard-idcard` | 身份证键盘 | [tmx-ui/表单组件/身份证键盘 xKeyboardIdcard.md](tmx-ui/表单组件/身份证键盘%20xKeyboardIdcard.md) |

---

## 子组件（不单独使用）

以下组件作为父组件的子项，不单独使用：

| 组件 | 所属父组件 |
|------|------------|
| `x-grid-item` | `x-grid` |
| `x-collapse-item` | `x-collapse` |
| `x-form-item` | `x-form` |
| `x-dropdown-item` | `x-dropdown-menu` |
| `x-swiper-item` | `x-swiper` |
| `x-steps-item` | `x-steps` |
| `x-drag-item` | `x-drag` |
| `x-waterfall-item` | `x-waterfall` |
| `x-snackbar-item` | `x-snackbar` |
| `x-barrage-item` | `x-barrage` |
| `x-cmarkdown-item` | `x-cmarkdown`（内部） |
| `x-picker-item` | `x-picker-view`（内部） |
| `x-slider-children` | `x-slider`（内部） |
| `x-tree-item` | `x-tree`（内部） |
| `x-devtool` | 开发调试工具（非业务用） |

---

## 类型导入

组件相关类型统一从以下位置导入：

```uts
import { 类型名 } from "@/uni_modules/tmx-ui/interface.uts"
```

常用类型：`TABS_ITEM_INFO`、`PICKER_ITEM_INFO`、`CASCADER_ITEM_INFO`、`FORM_RULE`、`FORM_SUBMIT_RESULT`、`XUPLOADFILE_FILE_VALUE`、`XUPLOADFILE_FILE_INFO`、`XACTION_MENU_ITEM_INFO`、`TABBAR_ITEM_INFO`

工具/配置从 `@/uni_modules/tmx-ui/index.uts` 导入：`xStore`、`xDate`、`xRequest`、`xColor`

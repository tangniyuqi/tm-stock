## 1.0.5（2026-05-21）
* 修复 Android 节点 PNG 透明背景被合成白底：png 仅绘制目标 View，不再使用 PixelCopy/根节点裁剪（jpg 仍保留屏幕合成以呈现父级背景）。
* 进一步：PNG 节点截图绘制时临时清除目标节点自身 background（uni 容器常自带白底），绘制完即时恢复；子节点 background 不受影响。

## 1.0.4（2026-05-21）
* 修复 PNG 截图白底：iOS 设置 `opaque=false`（jpg 仍为不透明）；节点/窗口均使用 `drawHierarchy(in:bounds)`，避免 `window` 坐标截取导致黑图。
* Android Canvas 回退路径从根 View 裁剪绘制，保留父级背景色。

## 1.0.3（2026-05-21）
* 新增 `format` 参数，支持 `png`、`jpg` 保存格式，**默认 png**。
* 修复节点内含视频/TextureView 等组件时截图黑屏（Android）：优先 `PixelCopy` 窗口拷贝，失败时回退 Canvas 并叠加 TextureView 画面；iOS 保持 `drawHierarchy(afterScreenUpdates:true)` 以保障常规页面截图正常。
* 新增错误码 `1007`：`format` 参数非法。

## 1.0.2（2026-05-21）
* **破坏性变更**：API 改为 uni 标准 opts 回调风格
* 已使用的项目升级前请阅读 readme 示例，不可直接替换旧版回调写法。

## 1.0.1（2025-07-27）
* 兼容鸿蒙
## 1.0.0（2024-12-18）
* 对屏幕截图保存(不需要权限),对指定截图进行保存,安卓,ios支持,web不支持.

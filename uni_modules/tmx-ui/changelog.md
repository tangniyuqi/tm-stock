## 1.1.22（2026-05-23）
* [重要] 本次适配了5.0.8编译器，因此4.87（较为稳定），还是最新的5.0.8都可以编译了。蒸汽模式（不管是JS引擎还是原生蒸汽引擎）由于官方在逐步开放，因此本次版本还未适配。
* [重要] 本次新增了原生图表库，x-chart，不依赖任何第三方库。纯原生图表。【演示位置：其它-原生图表】，性能非常流畅，这对于轻量图表使用者来说，再也不用厚重的webview百度图表了。
* 此图表完全Uts原生绘制，支持所有平台，所以可以放心使用，不必担心兼容问题。
* [重要] 本次更新还附带了Flutter版本，功能与UniappxUts版本一致，这是测试版本如果你有意使用flutter版本功能，请在vip群联系我，演示在文档顶部栏目进入
* 提供了（windows,macOs,Andriod,Ios,Web,Liunx版本的兼容适配）
* [重要] 使用了Opus4.6 max Fast,继续将剩余的组件逐一检查优化修复，性能提升等工作，将剩余非setup组件全部转换为setup组件。至此已经将组件库所有138+组件转变setup组件。为后续蒸汽模式适配奠定了基础。
* [新增] 新增了插件x-pinia-s，用于符合使用VuePina风格的全局store,并且可以支持指定store进行数据持久化。
* [新增]x-guid,x-guid-item引导组件，用于页面指示引导功能。
* [新增]x-blur-u，背景模糊插件，你可以搭配实现自由的毛玻璃效果。使用方式非常简单就当一个view使用就行了。见demo示例有演示。
* [重要] x-tabbar新增属性backdrop开启毛玻璃背景效果示例看demo使用，它依赖插件x-blur-u，因此你需要下载此插件到插件目录，并且要重新打基座才可以生效。因此使用了本组件后x-blur-u是必选插件且要打基座。
* [重要] 建议所有人更新：修复x-toast-s,x-loading-s,x-modal-s,x-tips-s可能的安卓内存泄露
* [重要] 有大量的更新和修复，建议全部用户更新跟进。
* 【重要优化】 x-screenshot-s，修改了函数风格为uniapp风格，原有调用方式不再兼容请参考文档重新使用，并且增加了图片格式，默认为png，允许节点截图透明背景图。
* [重要优化] x-webview-u,三个平台添加的相机和视频声音录制权限。请参考混合开发示例demo实现在网页上直接录制视频抓拍功能。编译demo时即可看到演示，演示的时候要注意
* 你的演示地址必须为https，如果你是按我文档采用的frp反代的话可以在宝塔中自己添加一个子域名，并申请https加密认证，然后添加在里面配置反代把你的ip地址解析到这个子域名。然后demo内的webview地址改成这个子域名这样就完成了本地开发代理，并且使用域名访问本地开发网站内容，并且有了加密这样测试时可以使用隐私设备（相机，录制等）
* [bug] 修复x-slider,x-slider-double在微信端的问题
* [优化] sign-borad签名版本增加笔峰属性：常规，钢笔，铅笔效果。
* [优化] tree-flat增加了双指捏合缩放（pc是滚轮缩放），双击恢复，默认初始渲染缩放居中。
* [修复] 多日历组件在微信上无限翻页后后再点选会出现偶尔闪烁，并优化性能。
* [优化] x-search优化在安卓/ios上在某些场景上如果快速切换会导致取消按钮出现支动画被打断，然后无法显示全。
* [底层优化] xTween优化和修复bug，防止在安卓上出现爆内存的风险。
* [优化]x-modal增加了disabled属性
* [优化]x-drawer增加了disabled属性
* [优化]x-icon中的font-size属性类型变更为string/number
* [优化]x-button中的font-size，iconSize,width,height属性类型变更为string/number
* [优化]x-sheet中的border,round,maring,padding等类型兼容类型```Array<string|number>|string|number```,现在允许你随意输入值，校验变得更为宽容。
* [优化]x-link优化减少在微信上64kb大小文件
* [优化]x-weekbar修复了卡的问题（不同平台不一样的，有的平台流畅，有的平台卡）。
* [修复]x-camrea-u，上个版本经过ai优化时丢失了符号(由于我本地编译有缓存导致没有发现)
* [修复]x-crypto-s，增加了hmacSha1加密，上个版本经过ai优化时丢失了符号(由于我本地编译有缓存导致没有发现)
* [优化]x-design，修改更新安卓依赖（使用离线包aar，并且升级最新）
* [优化]x-mqtt-s，修改更新安卓依赖（升级最新）
* [修复]x-file-s鸿蒙平台权限错误，导致文件无法选择。
* [兼容] hbx5.0.7在微信平台类型校验有个非常恶心的bug,即你定义类型类似export type = "x"或者函数等单行的，是无法编译通过。必须要改成{}对象式才可以编译通过。因此为了能编译通过类型是瞎写的和实际没有关联。只是为了能编译通过。警告就要管了。
* [优化]x-more，添加属性actionGap上下间隙，插槽action可以编辑操作区域布局结构。
## 1.1.21（2026-03-31）
* 【重要提醒】本次采用了地表最强Opus4.6 max Fast 累计针对底层api，工具，函数，方法极致性能优化提升，不同的组件提升5%-45%之间，最直观的是日历组件，weekbar等大数量计算的组件提升非常大。
* 您将得到强有力的澎湃动力,还有大量组件优化可能并未在下方展示。[下载体验](https://gitee.com/LYTB/tmui4.0/releases/download/1.1.21/tmui4x1.1.21.apk)
* （已消耗700美元优化）
* ***
* 【鸿蒙端】x-progress换到组合式后，ios端适配有问题已修复（具体原因是组合式比选项式性能差一丢）
* x-between-time修改样式
* 【微信端】x-upload-media内部计算尺寸的方法官方unisdk在后续版本中删除，导致排序失败。现已更新使用最新方法。
* 【非app端】x-grid-item.uvue,x-indexbar.uvuew修正非app下的css语法拼写错误
* 【微信端】修复x-swipec，事件类型在微信小程序上不支持，通过条件编译支持
* 【微信端】x-msg-notice修复在微信上官方接品已不支持setProperty，改用属性绑定代替。同步修复了click事件不触发 问题
* 【css类】自带的css类在源码代码项目中启用了一个vite.confing.ts用于对类提取，在发布阶段会扫描pages目录下所有的vue/uvue文件使用本类进行动态提取。
* 只有用到的才会被提取合并并打包。以demo为例启用后从之前的84kb -> 4kb 大小，减少了80kb。这对于微信小程序非常有用。如果需要的可以复制启用。
* 【bug】加密插件x-crypto-s,鸿蒙端aes加密ecb模式修复加密内容为空的问题。
* 【bug】x-mqtt-s,修复ios连接wss失败的问题
* [优化] x-navback，当用户填写pageErrorpath时，不再执行出错时的返回。
* 【bug】x-stepper，新增的属性autoFocus，focus忘记给默认值了。
* 【优化】x-input，confirm-type属性补上了。早期因此各种原因没放上去。现在如果有机型出现异常请自己填写为空值。
* 【修正】文档上的错误描述。
* 【bug】x-tabar文本未设置居中，导致跨平台时，不同平台可能出现文本不居中。
* 【bug】x-money，设置为-0.001时，会让值显示为-1.0
* 【优化】x-sse-s可能连接数据不完整
* 【优化】x-drag，x-drag-item,优化列表在滚动页面时的表现，当拖动元素在窗口边缘时自动滚动窗口，让被拖元素显示的更精准。
* 【优化】x-float-drawer，偶尔的跨平台时的差异滚动表现。
* 【优化】x-upload-media组件排序，删除了替换样式，改成了类似x-drag样的拖放排序，体验更好。
* 【bug】修复 x-animation 在微信小程序上失效。
* 【优化】优化x-finger手势的一些跨平台问题。
* 【优化】x-echart，在app和web端关于图表中options函数的处理方案，现在你可以使用web与app相同的格式，使用使用函数方法了，具体见demo
* 函数正常转为字符串即可，无限再手动转string了。
* 【优化】x-barcode，新增几种国际主流条码：ean13、ean8、upca、code128、code39、itf25（交叉二五）、codabar，为了应对更多场景开发。
* 【优化】x-tree-flat增加新的配置荐：曲线连接方式，现在允许直接和曲线两种连接方式。
* 【优化】绘画库ICanvas迎来了更强的性能优化，现在在跨平台中运行绘制更加的流畅。
* 【优化】x-tree,x-cascader相关组件得到了大量性能优化。
* 【优化】x-upload-file转换为setup模式，性能优化
* 【优化】x-virtual-list的性能。
* 【优化】x-image-resizer，提升性能，中心双指缩放更精准。
* 【优化】x-slider-menu转换为setup模式，提升性能。
* 【优化】x-vuei18n-s插件，请大家复制更新。
* 【优化】x-radio,x-checkbox相关组件优化性能提升。
* 【优化】x-picker-selected优化性能提升。
* 【优化】x-weekbar性能提升。
* 【优化】x-loading-s,x-tips-s,x-toast-s,x-modal-s四个常用插件优化安卓代码，可能出现的内存泄露。
* 【Bug】x-zip-s 修复安卓端压缩包可能在电脑端无法解压。
* 【优化】x-color-view在光谱模式时，更为精确的颜色转换位置。
* 【优化】xDate 修复几处bug及性能提升。
* 【优化】x-between-time样式警告
* 【优化】x-waterfall及x-waterfall-item性能和展示速度。
* 【重要优化】x-webview-u,适配了web端，即web套web。[线上demo](http://localhost:5173/70.%E6%B7%B7%E5%90%88%E5%BC%80%E5%8F%91/01.%E5%85%B3%E4%BA%8E.html#%E6%B7%B7%E5%90%88%E5%BC%80%E5%8F%91-%E5%85%B3%E4%BA%8E)
* 【优化】x-camrea-u,x-mlkit-scannig-s,x-mlkit-scannig-u，优化更宽泛的兼容，支持某些设备可能没有指定的相机设备分辨率，导致web启动设备失败。
* 【优化】 x-snackbar,添加了属性max-count，用于控制最大显示的数量。
* 【优化】x-tabbar增加了bottom，round,width用于控制更为个性化的底部导航。
## 1.1.20（2026-02-01）
| 机型 | sdk | 平台 | tmui版本 | 结果 |
|--|--|--|--|--|
| RedmiNote11 | 4.87 | 安卓12 | 1.1.20 | 通过 |
| Nova14标准 | 4.87 | Harmony Next6.x | 1.1.20 | 通过 |
| Xs max | 4.87 | iOS 18.x | 1.1.20 | 通过 |
| Xs max | 4.87 | 微信小程序 | 1.1.20 | 通过 |
| Web/H5 | 4.87 | Chrome,Edge | 1.1.20 | 通过 |
* [特别提醒]官方发布了sdk5.02版本，我在明年(2026-6月前)上半年前看官方的适配进度择机跟随推进。
* [重要提醒] 本次升级后，部分组件必须为4.87sdk
* 【重磅】发布x-webview-u原生插件，现在你可以使用内置的tmuijssdk.js嵌入到你的web页面中实现远程与宿主app融合的混合开发了。实现类似：淘宝等app变得更为容易简单上手
* 拥有完备快捷方法，权限控制。与宿主APP交互变得异常简单。sdk中已经为你封装了大部分uniappx内置api可在web页面中直接调用 。
* [安卓下载体验SDK](https://gitee.com/LYTB/tmui4.0/releases/download/1.1.19/tmuiJssdk.apk)；
* [开发文档](https://xui.tmui.design/70.混合开发/01.关于.html)
* 其它鸿蒙和ios需要自行编译，因此无法分发给大家体验，但体验是一致的。
* *****
* 【重要】多语言插件兼容sdk4.87
* [修复] 日历组件和多选日历组件，在click事件格式化时多打了个M。现已删除。 
* 修复兼容了多语言最新的4.86+的类型在web上判断兼容情况
* 修改x-overlay动画效果，这个一直处于试验的新动画效果。目前鸿蒙不理想，因此退回了早期的1.0动画。
* xActionMenu设置为sapce是底部增加安全区域
* 扫码插件安卓版本升级以适应16kb大小兼容。
* xAnimation组件重构为setup并优化动画函数。
* 新增useTool工具函数，考虑到好多人需要一个方便简单点的工具函数，因此推出，但会尽量简洁，不搞大而全，要控制大小，里面包含了些防抖，节流等常用函数。
* radio,checkobx优化统一函数
* collapse改造并优化为setup模式。
* x-barcode改造并优化为setup模式。
* x-circle-progress改造并优化为setup模式，增加属性startAngle允许设置不同的起始角度。
* 优化x-float-drawer
* x-calendar-multiple优化currentDate参数控制。
* 调整uplaodMedio的上传配置，之前核心上传功能写于3.89那会压缩配置官方没兼容，目前已经兼容，放开了鸿蒙，微信的压缩配置功能。
* 【x-pull-refresh下拉刷新时添加了属性，让列表无法再次重复下拉，直接本次刷新结束，防止重复刷新，避免请示多次。（此效果参考淘宝APP改进）
* 优化跨平台app，web,小程序兼容性。
* x-progress重构为setup，增加属性linear-color，用于渐变进度条支持。
* x-circle-progress增加linearColor，仅web,小程序支持，app不支持（为了提高app渲染性能）。
* x-popover重构为setup
* 优化x-upload-media的回显。
* picker-data,betweent-tiem,weekbar等组件，默认起始改成'xx-xx-xx 00:00:00'和'xx-xx-xx 59:59:59'避免时区问题
* 补上xheet的注释说明（上次改的时候删除了，导致文档也没了说明）
* xtabbar内部的一个函数写错了，导致安卓可能出现内存泄露。
* ****
* xDate我更新下文档：很多人会因为一个方法fromBetweenLongTime自我陷入错觉以为是计算 错误 。
* 比如两个时间相减是7小时，返回的结果是5小时前，好多人以为是错的，其实是对的因为7在返回的模板里面它在5-23小时之间，所以它就是5小时前，不会是7小时前。
* 因为模板中没有具体到每个时，而是间隔。如果你细粒度更精准，就自己提供下第二个参数的模板就行了。
* ****
* x-link上增加属性_style，用于想写style给内部text上,大家用插槽可以更自由的。
* x-switch,x-stepper添加了异步函数beforeChange，用于确认是否允许相关的操作。返回异步假取消用户的操作。同时添加了事件：blurOn，focusOn和属性:foucs，autoFocus
* x-msg-notice修复触摸问题。
* slider-menu优化，点击左侧菜单，右边置顶为当前节点（之前相同选中的节点下不会去再滚动）
* x-scrollx添加属性marginTop
* x-drawer,添加了一个插槽：bg，它在所有内容的底部，可通过它布局一些个性的内容在内容下边。比如背景图，背景渐变，其它特殊的设计要求，皆可实现。
* 弹层类插件：x-toast-s,x-loading-s,x-tips-s,x-modal-s等统一更新，使用了sdk4.65+推出的接口来避免与官方的弹层冲突导致无法显示。
* x-drawer的round支持了左右圆角，但默认如果为空值时不会取全局值，因此默认左右圆角是不起效的，但你如果填写了值就会起效（主要是为了向下兼容，而这个左右圆角也不好看，但为了各种设计稿的支持我开放了）
* x-dropmenu,x-dropmenu-item修复offset-top失效的问题，同时重构组件为setup模式。
* x-divider组件在最新的sdk 4.8x中 ios端出现文字断行，系sdk兼容，已替换写法兼容。
*  x-tabs的id索引，添加了联合类型string,number支持（升级后大家要记得清缓存再重新编译，否则会报错的）
*  x-upload-file修复在微信小程序上会触发两次change事件。(其它平台正常)
*  x-sticky组件改成了setup模式。同时修复在多页面时数据会串流。
*  优化了picker系列组件（共5个），本次在4.87上复现到了ios 的sdk bug已经全部做了特殊的兼容处理。本次更新后ios,鸿蒙应该可以正确默认选中选项了（之前ios会乱跳）.
* 针对sdk4.87,x-edit安卓失效，内容无法赋值的兼容，原因是这个版本的sdk的evaJs不需要对内容加引号了（之前的sdk需要的）
* 修复x-cmarkdown安卓渲染时无法解析换行符。（其它平台正常）
## 1.1.19（2025-12-10）
| 机型 | sdk | 平台 | tmui版本 | 结果 |
|--|--|--|--|--|
| RedmiNote11 | 4.86 | 安卓12 | 1.1.19 | 通过,1个组件功能受限 |
| Nova14标准 | 4.86 | Harmony Next6.x | 1.1.19 | 2-3个组件有异常 |
| Xs max | 4.86 | iOS 18.x | 1.1.19 | 通过 |
| Xs max | 4.86 | 微信小程序 | 1.1.19 | 通过 |
| Web/H5 | 4.86 | Chrome,Edge | 1.1.19 | 通过 |
* 【非常重要】暗黑细节调整较大，整体上比旧版本要更柔和（不刺眼），至于喜好因人而异，但不会更改回旧版本配置，如果你执着于旧版本请在仓库中下载旧的配置即可。
* 同时，主题色也再次细节化修改了，暗黑时主题色也作了调整（原先没修改），这次设计细节上的调整会显得更整体，更协调，更柔和，希望大家适应下。
* 整体上：这次主要是针对 感观细节，触觉反馈。在扣小细节。【升级时要重新复制x-vuei18n-s，static，hybrid目录依赖切记】
* 多语言插件删除===条件（安卓不允许，写习惯了）
* [新增组件，重磅] 新增x-cmarkdown组件,可以用来渲染纯标签的html文章或者纯Markdown或者markdown+简单的html标签混合渲染，此为实验版本，大家可以体验其实渲染效果
* 如果是纯html时，尽量标准，不要太复杂后台编辑器要删除不支持的属性标签等，纯markdown时，一定也要标准（也可在里面写一些简单的html标签，用来设置文本颜色样式等）。
* [新增组件] x-swiper-c，这是一个纯动效轮播组件，相比x-swiper组件的dom操作及元素过多时，性能更强加，建议在一个页面需要好多轮播时此时本组件，而且更容易做动效处理
* x-swiper由于是dom原生的图片加载大小等受视图限制动效不好处理。x-swiper-c就不一样了，动效是可以随时扩展和添加，且动画更为流畅，效果更多（后续官方支持了像素类型后可以做更多的酷炫效果）
* [重要插件] x-mlkit-scannig-s，兼容鸿蒙原生系统，采用鸿蒙的扫码识别服务，需要真机测试，界面等与其它平台保持UI一致性，另外与官方的扫码是有区别的
* [重要组件] x-camera-u原生相机组件，兼容鸿蒙原生系统，可以录制视频，抓拍。【目前还未完善鸿蒙建议用来拍照图片可以，不建议录视频】
* 官方的扫码是调用华为自带的统一服务，界面不可定制，而我的是采用了相机+自定界面，因此可以根据需求修改界面，灵活性更高。
* [修复] 微信手写模板无法清除了（sdk 4.76+导致）
* [修复] canvas库，导出图片时在微信上报错，现改为使用base64方式，可以最大程序减少兼容性。但需要插件代码库中的x-base642file-s插件。使用时记得一并复制到插件目录。
* [修复] drag给微信加上stop事件阻止页面滚动。
* [修复] x-more在1.1.18的sdk上打不开了。
* [优化] x-ocr-s,方法已全部改成为DClound函数风格方式调用，请升级前作好备份，如果不愿意更改不要升级。另外sdk调整升级了，具体见插件文档介绍使用。
* [优化] x-tabbar，在app,web端时，凸起造型时，可能各端会有0.5px-0.8px对齐问题，现已找到了原因，是各平台对小数点绘制精度不同造成的。现统一了精度。
* [升级] 图标升级为remixicon到4.7，对应的x-icon升级及static目录的字体也要相应升级。(仅对非微信端)
* [优化+修复] x-sqlite-s,鸿蒙端增加了部分函数的路径及文件名校验，防止使用者没有按跨平台规范使用。修复web异步加载问题。
* [优化] x-cell新增了darklabelColor属性。
* [修复] x-markdown暗黑下表格及code背景没有为暗黑转换问题
* [优化] x-switch-slder增加属性longTimePress长按事件。新增：showBottomBorder控制显示下边线。
* [优化] x-float-button,x-switch-slider将点击事件改为更灵敏的5ms之上视为点击。[见此讨论](https://git.tmui.design/tmui/tmui4xIssue/issues/77)
* [优化] 日期组件增加showClear，showFooter及isSameDay(是否允许选择同一天，入驻酒店场景需要)
* [优化] 修改了主题色值，以跟上新的时代。
* [修复] 全局底层导航高变量拼写的错误tabbarHeight（请使用的人注意纠正下。）
* [修复+优化] 修复和适配了表单在鸿蒙上的校验及labelFontColor在暗黑是不生效的问题。但要注意一个校验的问题我已提交官方[issue](https://issues.dcloud.net.cn/pages/issues/detail?id=23107)
* [修复] 在新的4.8x sdk上表单验证出现嵌套form-item在view下无效，及验证,动态字段，动态rule问题，这个牵涉修改到多个组件，现已修复见[issue](https://git.tmui.design/tmui/tmui4xIssue/issues/111)
* [修复] xImageGroup ,添加了个属性maxCount用于显示最大数量,同时添加了一个默认的slot插槽，用于自定布局更多的界面。
* [修复] xWeek，当颜色值为空时，ios端的sdk会有bug会解析成白色。现在以rgba(0,0,0,0)代替空值。
* [优化] 日历组件，click事件统一格式为，x-x-x。
* [优化] x-picker-datae某些日期出现不全
* [优化重要提醒] picker类组件，我全部重构了，以适应新的sdk，但目前ios真机端，打开后出现未选中的情况我还无法优化到最佳的样子，这个需要官方配合
* 重构这个组件，我才好进一步优化。重构后，放弃了子组件x-picker-item，同时样式只能选中高亮模式时无圆角间隔了（为了妥协我只能舍弃了）。另外ios暗黑样式白的问题
* 也需要官方配合（如果新的sdk修复了自然就正常）。总的来说这个组件：在ios上是有问题的，需要等sdk修复和优化。
* [修复+优化] x-tabs 修复在未开房dotTtype时，textStyle丢失。同时增加属性lineWidth，用于控制底部指示的线宽（如果设置宽和高相等可实现圆的样式指示点，可自己探索更多可能）
* [修复] x-vuei18n-s在app上切换时，语言不会即时响应渲染。（请大家在升级时务必重新复制这个插件到你的插件目录，删除原有的）
* [修复] x-modal类型批注错误。
* [优化] x-weekbar,增加了：disabled用于控制禁用用户滑动操作，同时优化为当vmodel切换时，禁用动画，当手势操作时再开启动画。
* [优化+修复] x-float-drawer在安卓上的体验表现，同时修复了手动打开和关闭事件时，未主动去同步状态，导致可能外部状态不一致，从而导致打开错乱。
* [修复] x-edite在微信平台上无法获取到正确的内容。
* [修复] x-switch-slider非微信平台时，当滑动边界时，sdk不会触发动画事件，导致无法同步关闭和打开状态。
* [重要] x-virtual-list，我删除了1.1.18针对鸿蒙的子协议滚动功能，原因等了官方两个月了，始终修复不好，干脆不用nestScroll了。代码在本版本中直接删除了，以全至少在鸿蒙中能显示使用。
* [优化] x-waterfall瀑布流组件，优化与之前不太一样了，建议大家迁移时看清文档和示例全身，主要是让组件可以被嵌套及添加悬浮置顶功能，这样组件在被放置在页面底部时，可以滑动到组件时自动置顶顶部并执行列表组件。
* [优化] x-search,添加autoFocus和disabled，用于自动弹键盘及禁用。修复在鸿蒙平台默认提示文字颜色为暗黑时为黑色（此为sdk bug）
* [优化] x-rolling-number,添加了属性fontStyle用于控制文本样式覆盖。
* [修复] x-tabs,在鸿蒙平台默认激活项时，指示条未显示。
* [优化+修复] x-sign-board优化在鸿蒙平台连续性问题，及修复在鸿蒙平台下导出的图片变形（此为sdk bug，现采用其它方案导出图片，非鸿蒙平台不变）
* [修复] x-drag,在鸿蒙平台下初始时尺寸不正确。针对最新的sdk 4.86做了兼容。
* [优化] x-popover,在增加非透明值的背景色时，增加背景动画渐显，另外针对不同平台指示三角形在鸿蒙平台会出现0.5px间隙的修复。
* [修复] x-calendar-view和x-calendar-multiple已经在sdk4.86+上兼容，可以正常使用了。此处在鸿蒙平台上作了特殊兼容处理，因为sdk布局bug迟迟未能修复，目前是曲线救国，后期如果sdk完善了会改回与其它平台一样的方案。
* [修复] 鸿蒙平台（真机）x-picker-selected输入框在暗黑时，颜色未转白，导致看不清。
* [修复] 鸿蒙平台（真机）x-input-tag，输入框在暗黑时，颜色未转白，导致看不清。
* [修复] x-dropdown-menu在web平台下，如果为static时，同时取消了系统导航，会导致位置不对。
* [修复] x-input的属性rightIcon未使用。
* [优化+修复] x-upload-media，修复在鸿蒙上显示异常（原因未知），另外修复了一个长期存在的手势bug(但不影响使用，影响使用过程的日志美观，会有意外出现日志错误。)
## 1.1.18（2025-09-25）
* [版本] 编译到app/web/h5请可以使用4.76+(鸿蒙还有部分组件有异常)，微信小程序请务必使用4.81+
* [重要] 整体组件细节样式调整，颜色计算逻辑调整，使得整体更有精致感（请使用细腻的屏幕重新感受对比）。主题要更偏年轻化。,css库中添加了对应的状态颜色如text-error,保持与主题匹配
* [优化] 大部分示例demo页面重构为setup版本。优化文档显示。
* [重要] x-echart,重构为setup，同时废除属性opts,请使用ref方法setOptions来设置数据，经过测试4.76+上通过组件属性设置过多数据的参数会导致失效，我也莫名其妙。
* 同时图表已经升级到最新百度图表6.0.0版本，经过我观察样式和主题默认颜色有些许变化。升级时微信请重新复制demo页面中的echart.esm.min.js(原先是echart.min.js)，非微信请复制hybrid到你目录中
* [重要] xtext修改单词highlight，因此你的highlight，highlightReg，highlightStyle都要修正为这三个，请一定注意修改。
* [新增插件] x-chinesetopinyin-s可以将中文转换为拼音（含声调注音及不带注音的两个属性）（全平台支持）
* [新增插件] x-texttovoice-s 文本转语音播报功能（小程序不支持，其它平台全支持）
* [重要优化] 画布ICanvas API库已经对鸿蒙兼容适配了，增加了画布30篇文档使用及介绍，同时增加了示例案例：语音录制动画，海报绘制，自动图形排版，新增图形二维码。目前优化已经基本可以适配商业场景：如海报分享图，动画绘制等业务场景。
* [重大优化] x-table,现在支持子元素列之间的合并（它是向右合并吞并下一列数据实现，因此它的下列会自动隐藏不显示），
* 同时单元格高不再固定高改为最小高如果出现大量数据会自动断行（其它列自动上下撑开对齐本行高），同时添加了行多选checkbox功能
* [重要优化] x-tabbar优化了canvas绘制时的逻辑
* [优化] x-tree组件现在允许最后一级（非异步且没有子节点的场景时）点击选中切换状态。
* [优化] 全局config中增加了：fontColor，fontDarkColor分别对应默认的文本色及默认的暗黑文本色。
* [优化] xfinger,对外暴露更多slot参数。
* [优化] xDrawer重构为setup版本,同时修复因为版本差异的web端嵌套的问题。
* [优化] xModal重构为setup版本
* [优化] xOverlay重构为setup版本
* [优化] xText重构为setup版本
* [优化] xRadio重构为setup版本,相关值类型改成为:string|boolean|number
* [优化] xRadioGroup重构为setup版本,相关值类型改成为:string|boolean|number
* [优化] xCheckbox重构为setup版本,相关值类型改成为:string|boolean|number
* [优化] xCheckboxGroup重构为setup版本,相关值类型改成为:string|boolean|number[]
* [优化] x-action-menu重构为setup版本,并且允许在不同平台嵌套使用
* [优化] x-action-modal重构为setup版本,并且允许在不同平台嵌套使用,修复多语言时默认为中文的问题
* [优化] x-alert重构为setup版本
* [优化] x-sheet重构为setup版本
* [优化] x-float-drawer重构为setup版本
* [优化] x-tag重构为setup版本
* [优化] x-tree-flat思维导图重构为setup版本，并在opts配置项中增加属性layout ?: 'horizontal' | 'vertical',用于左右，和上下排列（之前只有左右）
* [优化] x-text-cloud重构为setup版本(并已兼容鸿蒙Next),并设置新的散射样式。
* [优化] x-button重构为setup版本，loading icon大小同步图片大小一致
* [优化重构] x-float-drawer浮动面板优化重构，由于sdk版本导致在ios和微信端可能无法流畅滚动，因此重构了，已经在5个平台中测试较为流畅。
* [优化+修复] x-weekbar,优化体验问题，当设置了seekDay时，如果当前日期默认不在偏移范围内，可能会导致当前的日期不可可视范围内。
* 同时添加了同日历组件一样的状态属性，类型格式都是一模一样的。方便给日期底部添加对应状态提示。然后修复了拼写错误造成不能赋值，优化允许动态修改start,end
* [优化] x-search重构为setup版本,并新增属性showCancel
* [优化] x-table重构为setup，同时兼容一个sdk bug,即动态将list-item设置显示和隐藏会触sdk bug，大家要自己避免这个list-item的bug（后续可能官方 什么 修复）.
* [优化] x-cell重构为setup版本
* [优化] x-code-input重构为setup版本
* [优化] x-countdown重构为setup版本
* [优化] x-empty重构为setup版本
* [优化] x-float-button重构为setup版本
* [优化] x-grid重构为setup版本
* [优化] x-grid-item重构为setup版本
* [优化] x-icon重构为setup版本
* [优化] x-radio-button重构为setup版本
* [优化] x-rate重构为setup版本
* [优化] x-rolling-number重构为setup版本
* [优化] x-badge重构为setup版本
* [优化] x-virtual-list,兼容一个ios sdk的一个bug，即ios在设置滚动为子协商时会无法滚动（仅ios有此sdk bug，后续可能官方 什么 修复）
* [优化] x-slider-children重构为setup版本
* [优化] x-slider-tree重构为setup版本
* [优化] x-msg-notice重构为setup版本，同时增加了上和底拖拉时的反向操作阻尼反馈，让体验更完美。更新了示例效果。同时增加了对pc的鼠标手势支持。
* [优化] x-notice重构为setup版本
* [优化] x-snackbar重构为setup版本
* [优化] x-more重构为setup版本
* [优化] x-loading重构为setup版本
* [优化] x-input-tag重构为setup版本
* [优化] x-image-group重构为setup版本
* [优化] x-code-input重构为setup版本
* [优化] x-card重构为setup版本
* [优化] x-keyboard，x-keyboard-car，x-keyboard-idcard，x-keyboard-number重构为setup版本
* [优化] x-snackbar-item重构为setup版本,优化了动效。
* [优化] x-backtop重构为setup版本,同时增加了disabledPageScroll,scrollTop属性，以实现局部返回置顶功能，具体见demo实现案例。
* 【修复】日历相关组件在同时设置start和end会产生边界判断问题。
* 【修复】x-between-time未对外使用format格式同步数据。同时添加默认插件数据startVal，endVal,show
* 【优化】x-barcode对原生的条形码改为code-128标准条码见：[标准128](https://barcode.tec-it.com/en/Code128?data=ABC-abc-1234)
* [优化] x-qrcoder添加新的属性wifi字段，用于生成自动连接的wifiQr码，详见demo示例页面展示。
## 1.1.17（2025-09-03）
* 【声明】本次更新中在1.1.16版本中提到的鸿蒙Next相关插件和组件，在本次版本中并未更新兼容。请关注后续版本。
* [适配] 本次版本适配了xSqliteS数据库插件，从1.1.17版本开始可以用于鸿蒙原生应用了。
* [优化] xDrawer动画放弃了官方的动画结束@transitionend事件，改用setTimeout，官方 的这个接口在不同机型和设备上有不同的差异，慎用。
* x-darg-item 写错了一个语法。会在鸿蒙编译的时候无法编译通过。并在插槽上添加数据：order，用于需要显示自身索引位置的场景。
* 优化demo页面展示，同步组件的使用方式。
* [优化] x-form新增支持嵌套字段校验如{a:{b:""}},需要form-item上属性filed="a/b",关于'.'连接符安卓sdk会在4.86+上支持
* [优化] x-tabbar,增加事件doubleClick。[issue](https://git.tmui.design//tmui/tmui4xIssue/issues/11)
* [修复] x-button，loading图标没有跟随字体颜色。[issue](https://git.tmui.design//tmui/tmui4xIssue/issues/7)
* [修复] x-tabs,中激活动态字体大小与普通字体大小不一致时导致的定位偏移。见[issue](https://git.tmui.design//tmui/tmui4xIssue/issues/3)
* [修复] x-slider-menu暗黑切换项目背景在ios端有兼容性。见[issue](https://git.tmui.design/tmui/tmui4xIssue/issues/12)
* [修复] x-drawer多语言设置时变量出错了。见[issue](https://git.tmui.design/tmui/tmui4xIssue/issues/16)
* [优化] xDate新增函数getBetweenDate，用于取start,end之间的比较，如果在范围内返回本身，如果在范围外取start或者end,返回的值始终在起始之间。
* [修复] xBetweenTime ,限制start,end后，如果点快捷选择时会超出限制的时间区间。[issue](https://git.tmui.design/tmui/tmui4xIssue/issues/18)
* [修复] xWeekbar在提供了start和end后未限制日期[issue](https://git.tmui.design/tmui/tmui4xIssue/issues/20)
* [增强] xWeekbar新增属性seekDay，可以控制一周的第一天是任意一周的某一天（0-6）
* [修复+优化] 修复xIndexbar索引组件，使用flex:1时无法自动填充父容器（导致无法全屏或者局部），为组件增了属性：customSliderBar用于自定右侧菜单，新增插槽：top用于自定义顶部自由布局，当右边滑动时会自定定位到top插槽位置类似微信通讯录。
同时重新编写了demo索引页面，仿微信通讯录示例。
* [修复] x-money,值为0时的千分位问题[issue](https://git.tmui.design/tmui/tmui4xIssue/issues/21)
* [优化] xGrid组件新增round，showBorder等相关属性，以显示边线和圆角相关，子组件相应增加order,同时两组件重写为setup组合模式了。demo示例页面增加了更多的示例
* [增强] 相关日历组件增加了属性seekDay用于设定月面板中周的第一天是0-6（周一至周日）中的哪天为开始。比如设定6，表示以周日为起始第一天排序。
* [增强] 二维码，新增了一个属性pdRounded，用于控制是否让三个定位变成个性化圆形，同时mode增加了可选模式：vertical竖形胶囊和星星,看起来可爱一点。
* [增强] 日历组件增加了dateStatus设定日期状态指示，并更新了示例页面为组合式代码。
* [修复] betweentTime组件因多语言设置忽略了用户的format格式的输入导致输入框没有按用户的格式显示。[issue](https://git.tmui.design/tmui/tmui4xIssue/issues/23)
* [优化文档] 对技术文档重新梳理，并启用了新文档页面。
## 1.1.16（2025-8-17）
* 已有项目请谨慎更新！！！，因兼容鸿蒙部分组件可能更改了结构。
* 【重要】请务必使用4.75sdk+，本次更新鸿蒙兼容，并不代表完全兼容，以下有部分组件还要继续完善，还有一些插件未提及可能也无法使用，但大部分是可以使用的。其它问题在后续中继续完善。
* 适配原生鸿蒙Next(5.1.1/5.0.3+ api 13+),由于鸿蒙的插件写成了har包，因此对应的Next平台的插件部分源码将不开放，其它平台遵循对vip开放原则。
* [不兼容鸿蒙组件]： 日历，swiper轮播，：这几个由于sdk问题和排查较久，先让我和官方沟通。
* [鸿蒙体验不佳]:x-slider-tree,x-watermark,x-text-cloud,x-barrage，x-sign-board：这几个由于sdk问题和排查较久，先让我和官方沟通。
* [不兼容鸿蒙插件]:x-mlkit-scannig-s,x-mlkit-scannig-u,x-camrea-u:这三个由于我没真机，暂没去兼容等我真机。
* [优化] x-betweent-teim增加属性widthCoverCenter，同xDrawer属性。
* [优化] x-cascader被重构，以兼容鸿蒙Next;
* [插件] x-svg-u 变更较大，不再需要导入组件，可以直接在页面上写标签使用，已变更为标准原生组件，请注意适配。
* [优化] 相机的插件和组件，针对性的为x5内核的浏览器（微信），优化，以防摄像头切换和自动悬浮全屏，脱离文档。
## 1.1.15（2025-07-16）
* 【非常重要】本次更新是破坏式更新，主要是内置组件实现了多语言，并且需要安装多语言插件。app.use入口类型改成，结构改变，具体见demo或者入门安装文档那比对下。
* [新增] 新增多语言插件x-vuei18n-s，可集成到其它应用中，在tmmui4x项目集成时，一定要复制在运行插件目录，组件库自动安装集成。
如果是非tmui4x项目请按照插件内的文档自行集成。本插件用法与原官方插件使用基本一致，无学习成本，可参考组件使用。
* [修复] x-image微信端条件编译问题及不再使用下载接口（仅微信端）。
* [优化] x-tabs在4.71sdk下闪烁。官方已经在4.74sdk中修复
* [优化] x-echart在4.71 sdk web中出现init两次触发。
* [优化] x-pikcer4.71sdk 闪退（4.72修复）
* [优化] x-input 4.71sdk黑边，（4.72修复）
* [修复] 4.71sdk xEchart在web端事件被执行了两次。
* [优化] 两个日历组件x-calendar-multiple和x-calendar-view在hbx sdk4.74+上无法点击了，追踪的原因是sdk bug导致，但其它平台不受影响，为了兼容ios这边取消了视差动画效果，其它平台动画维持。
* [优化] x-slider兼容hbxsdk 4.74+,相同值同时或者反复赋值背景动态条无变化，还是旧的视觉。此bug在4.74上引发。
* [修复] sdk4.74+可能导致radio,checkbox组件无法选中的问题。
* [修复优化] x-picker-selected 当采用远程搜索数据时因为list可能为空，会导致选中数据被清空，现改成当用户一直在搜索变动list时，会将所有查询的数据缓存到本地供查询使用。解决赋值为list空时
* 以上仅对localSearch=false有效。
* [优化] x-qrcoder，添加logoBgColor背景
* [优化] picker,picker-city,picker-date,picker-selected为了更好的回显在默认触发插槽增加了label数据插槽数据，可以实时回显，以增加便利性。（与之前的vmodel:modelStr不冲突，这个是用来非插槽控制时便于回显）
* [优化] x-input增加属性isLink和rightIcon便于作为表单中配合其它组件作为静态展示显示，比如和picker配合点击触发，再配合上面的插槽数据回显，可有效减少变量的使用。
* [优化] x-cell右侧不显示任何信息时，取消间隙的设定。
* [修复] x-calendar-view，修复单选日历选中出错的问题，同时优化，不允许跨月点击，只允许用户翻页（滑动翻）选中当前面板日期
* [修复] xtips插件在执行关闭时未执行参数中close回调
* [优化] xmodal兼容修复contentPadding参数（向后兼容）
* [优化] weekbar组件增加事件swiperChange，滑动切换周期时，触发切换事件，并返回当前的7天时间数组
* [修复] xmodals,xtoasts，xloadings插件，因为异步创建的全屏组件，导致后续如果连续打开，无法自动关闭上一个弹层。
* [优化] x-form增加ref函数checkAsyncVaildStatus
* [修复] x-popover修复在微信上事件丢失。 
* [优化] x-swiper优化拖动的流畅度

## 1.1.14（2025-06-15）
* 【非常重要】此版本仅可sdk4.71+(含)以上编译，如果你的项目在4.71以下，不要升级本版本，如果一定要升级请选择性升级不可全部。
* [新增] 新加了个xVirtualList超高性能虚拟列表数据，这将是处理列表的不二之选，在app端要比listView等官方组件性能强悍的多，助力安卓性能瓶颈。
* 首页探索页面已经采用了xVirtualList虚拟列表实现。对于大家处理数据列表的展示，固定高的列表展示非常顺滑
* [文档ai] 本站已在根目录存放了llms.txt，如果使用AI搜索可以直接提供链接https://xui.tmui.design/llms.txt文件喂给ai快速检索。
* [优化] 基于最新的4.71sdk，插件已经开始支持app.use(xui)或者app.use(xui，{配置参数})，现在不需要在app.uvue中使用函数配置参数了，可以直接通过入口初始化了。
* [优化] x-form添加了ref方法clearValid用于清除验证状态
* [优化] x-row,x-col使用setup重写。
* [优化] 基于最新的4.71sdk优化并开放了x-calendar-multiple，x-calendar-view日历的无限滑动切换功能，并且配置了readyOnly（默认开启，如果不要，可关闭以提升体验）以支援pad等10年前的低端机支持性能上的提升。
* [修复] xDate,修复31，29号等日期时可能造成异常错乱。从sdk4.71+开始，您可以自由的使用本xDate库了。
* [修复] xModalS支持html渲染后，未兼容安卓6及以下版本。同时添加按钮分割线样式，以应对不同的样式偏好设计。同时修复ios端渲染html后颜色设置失效。
* [修复] x-navbar未使用参数backErrorPath
* [兼容] x-mqtt-s在sdk4.71下无法编译，修改后此插件仅可在4.71+以上编译。
* [新增] x-input-number新增一个数字输入框组件，用于数字场景输入
* [优化] 加密插件x-cryptop-s增加了两个函数aesEncrypt2，aesDecrypt2用于对传参及加密解密输入和输出全程为Uint8Array格式数据，解决转换的问题。
* 但由于IOS端在插件内直接使用Uint8Array处理数据会闪退（我可以确认是sdk bug)，因此在传递参数是使用Arraybuffer,Uint8Array.buffer即可，加密和解密返回来的是number[],转换为Uint8Array使用Uint8Array.from(number[])
* 具体见加密和解密demo示例页面的操作。
* [修复] xDrag在拖时，如果停止拖放的位置在禁用的项目位置时，会覆盖禁用的项目。正确的应该是回退到原位置不允许覆盖。
* [优化] x-cascader单独开放了内部类型，修改了文档注释。顺便优化了下切换体验。
* [修复] xModalS,xloadingS插件内存管理。
* [优化] x-modal组件间距视觉改进。
* [优化] x-drawer在内容顶部添加了一个插槽：contentTop用于在顶部制作额外的营销类布局。见demo
* [优化] x-picker-selected开放了实时本地搜索（之前考虑会卡一直没放开）
* [变更] x-modal变更了动画效果
* [优化] x-picker系列组件开放了同Drawer的相同属性widthCoverCenter，以控制是否需要开启宽屏适配。
* [优化] x-picker和x-drawer开放了customWrapStyle属性用于自定义背景容器的外观可见两demo示例，以方便更多的设计外观。
* [优化] x-tabs增加了属性textStle及textActiveStyle文本样式。
* [优化] x-network-change,增加isNetworkAvailable和getNetworkType函数具体见插件文档。
* [优化] x-modal踢除了官方@transitionend事件，用timeout代替，近1年来此api接口经常在不同机型上概率性不执行，导致有些机型无法关闭弹层。如果此修改不影响后续，未来所有组件将逐步删除。
* [优化] x-markdown，新增对LaTex数学公式的支持,安卓，IOS,web直接支持，微信小程序请一定仔细查看文档使用才可支持。效果见demo示例
* [修复] x-collapse-item未注册click事件，导致重复触发。
* [修复] sdk不止从可起，x-popover微信端不管采用什么查询方法得到的悬浮节点bounds总是不准确，具体见文档说明。为了保证微信准确，内容最好不要动态更改宽和高内容（如果确实要变动可以vif或者刷新整体组件）。
* [优化] x-popover增加属性showTriangle及相应的triangle-color，triangle-dark-color显示指示三角及对应的颜色。默认不显示（兼容旧版本）
* [修复] x-drag在app端，如果跨行，拖放到禁用项目上，有机率会造成被放置到禁用项目上
* [优化] x-grid,x-grid-item分别增加了textDarkColor属性
* [修复] x-swiper自动轮播时，change事件未触发 。
* [优化] x-table添加属性hideHead用于控制是否显示头。
* [修改] 1.1.13中加入的字号响应，已经删除，经测试x对于computed函数在text内赋值性能影响太大。
* [优化] x-image逻辑尺寸app与非app单独条件编译分开，避免微信经常忘记加域名下载白名单。
## 1.1.13（2025-05-29）
* [祝各位2025端午节快乐，家庭团圆幸福，阖家欢乐，此敬！]为了节日期间防止程序员们加班劳累特别更新本版本，增加一些便利属性，在最后两天好上线产品。
* [重要] 请使用hbx sdk4.53,4.57,4.65-4.66,4.67dev版本编译应用，其它版本不要偿试。（如果你从4.53升级出现编译不了，请卸载应用并全新安装）
* [优化] x-float-button优化显示时机，以兼容pad等低端机设备
* [优化] x-form新增了属性rules,要求属性：**Map<string,FORM_RULE[]>**，其中的string为form-item中的field,可以将整个表单的校验模块化统一组织管理提供
* 为了向下兼容，之前组件上的form-item上的rule同样有效，两则有一边命中校验的key即可校验。如果两边都提供将会合并校验为数组一同参与校验。
* 同时增加了属性watchValidStatus是否开启实时同步校验状态（非必要不建议开启），需要通过v-model:modelValid绑定获取实时的校验状态，用于外部指示校验按钮状态等场景
* [优化] x-form-item增加了属性contentStyle默认插槽的样式自定，特殊的情况可能需要直接布局，可以减少外部再套一个view节点的。
* [优化] x-upload-media优化了数据同步转换，防止响应式传递，同时增加了属性align用于项目左对齐还是右对齐。修改了相关样式。同时内部的xImage被替换成了官方的image因此没有了加载状态指示及错误图片指示了
* 更改原因：在表单中，如果过多自定节点组件会影响表单渲染性能。
* [优化] x-switch-slider增加属性eventNone以控制内容层是否响应事件。
* [修复] x-drag-item在ios下异常显示（其它平台无影响，主要是sdk平台差异导致）
* [优化] x-switch添加activeIcon，icon属性用于显示开关激活和未激活时的按钮图标，不提供不显示。
* [优化] xRequest，提供了loadToastText：加载时提示文本,successToastText：成功提示文本,errorToastText：出错时的提示文本，相关值属性
* [优化] 文档添加了微信分享文档操作方案，同时demo首页也添加了微信分享示例。
* [修复] x-collapse-item，我多写了个ref名称导致高异常。
* 其它的优化及修改。
## 1.1.12（2025-05-24）
* [重要] 请使用hbx sdk4.53,4.57,4.65,4.67dev版本编译应用，其它版本不要偿试。（如果你从4.53升级出现编译不了，请卸载应用并全新安装）
* [重要] 升级，请重新复制hybrid文件夹内的文件，重新复制插件目录文件。
* [优化] xRequest请求优化了根据最新的sdk简化了逻辑判断，配置中新增了hostUrl(请求的时候可以随时配置，这样可以多域名，多模块请求),cacheTime：缓存时间默认1分钟
* useCache是否启用接口缓存（只对post,get进行缓存）,最多在本地缓存20条接口，超过后远到近递减替换，以减轻字典类的接口重复的请求,直接从根上解决需要数据缓存的接口。
* [优化] xRadioButton,如果提供icon,title为空时,让其它间隙为0,以便支持纯图标选择样式组件,同时增加了textStyle自定文本样式。
* [修复] xPickerSelected组件在兼容微信时,丢失了底部操作条. 
* [优化] xPickerSelected增加了一个属性isRadioMode:当设置multiple为false时,是否允许为单选唯一模式,即不允许取消唯一项,意味着一旦选中一项就无法清空或者取消.
* [优化] xModalS,ios,安卓,web，内容现在可以渲染html内容,不要太复杂.安卓和web会拦截a标签,ios目前拦截不了会直接跳转外链.
* [优化] xModal新增属性beforeClose属性,可用于在确认按钮之前执行异步函数,是否阻止关闭弹层.
* [优化] xDrawer新增属性beforeClose属性,可用于在确认按钮之前执行异步函数,是否阻止关闭弹层.
* [优化] xCheckbox增加round属性,用于控制圆角,默认为4
* [优化] demo首页删除了浮动，改为左侧滑菜单演示效果。
* [优化+修复] xDate，设置顺序调整，可能带来的不准确。以及修复历史遗留问题只能选中到1970之后。
* [优化] x-picker-selected为空数据时，可能出现的位置偏移。
* [修复] x-sqlite-s 修复安卓端执行插入数据时，可能会被执行两次。修复安卓/ios端不能使用应用的uni.env.USER_DATA_PATH目录及自定二级目录，在安卓端执行COUNT(*)时类型转换错误。
* [优化] x-collapse-item的异步获取问题。兼容app sdk4.65+,微信端兼容优化。
* [修复] x-file-s下微信小程序端类型爆错误
* [修复] x-scrollx下微信小程序端在hbx4.57+以上失效。
* [修复] x-avatar-group下微信小程序端在hbx4.57+以上爆红问题和异常error(但不影响使用，但看着难受)
* [优化] x-tag在微信小程序端样式延迟响应（其它端正常）
* [优化] x-calendar-multiple日历范围组件全新重构,已开放无限滑动切换，切换效果更炫，如果本次更新x-calendar-multiple反响好，下次就会依此对x-calendar相应的更新
* * 我已经请求了官方优化，请关注官方的4.67版本再对比性能目前4.57-4.66没有太大变化，如果过多样式，还是有点延迟，以上说的仅对安卓说明。非安卓比较流畅。并增加了头和尾插槽
* [修复] 修复x-tabbar异形外观时，镂空在安卓端会有0.5-1px的差异
* [优化] x-drag-item添加disabled属性，可以禁止某项拖动，并保证该项被固定顺序不会被替换，对于特殊的排序要求非常有用。* [优化] x-drag-item添加disabled属性，可以禁止某项拖动，并保证该项被固定顺序不会被替换，对于特殊的排序要求非常有用。
* [优化+修复] xoverlay,x-modal更新了在web端的显示机制，并修复了xoverlay在web端重复打开可能的报错显示问题。
* [修复] x-action-menu因兼容安卓sdk缺陷时，导致其它平台item-click被重复执行。
* [修复] rolling-number在不平台时，如果动画执行中更新，可能导致动画无法继续，导致数据更新与实际值不符。
* [优化] xToast,xLoadingS,xModal优化为全屏覆盖。
* [优化] x-swiper重构部分，及手势状态，让其在页面内滑动与页面滚动减少手势冲突，并且重新开放了item事件click，同时也保留了父级的click事件向后兼容。
* 同时增加了loop属性及事件dragLastEnd及显示showLastView，配合后，可以实现拉到最右侧拉不动时，显示最右侧的内容，并触发拖拉事件（仿头条首页推荐视频拉到最右时触发继续拉触发到视频页面）
* [优化] x-popover增加了上下边界检测，如果设置为下显示时，下空间不够会自动转为向上展示，其它情况类似。
* [优化] x-crypto-s加密安卓调整函数以支持兼容安卓5.0及更低版本安卓（pda设备好多比较低的版本）
* [优化] x-float-button新增事件longpress，用于支持长按时触发。
* [优化] x-sse-s 参数修改，支持header，并增加demo示例页面，请参照demo修改你的使用方式。
* [优化] x-float-drawer,标题高亮修改为仅触发标题时指示高亮，底部拖拉修复可能出现的错位，并设置为点击头就置顶滚动。
* [修复] x-text微信端在兼容本身sdk bug时，导致插槽事件丢失，无法触发click(通过label不受影响)
* [优化] x-echart在最新的4.57+sdk上带有函数的配置失效，为了应对不同平台频繁兼容问题，请参考我的demo对有函数的opts配置分平台按格式写。目前demo演示了app上带了函数格式化的配置。
* [优化] x-upload-file，web端向后端传递文件名。
* [优化] x-color-view,优化了样式，新增showAlpha控制显示透明度。新增ref函数getAlpha，getColor，getColorNoAlpha
* 其它优化和修复
## 1.1.11（2025-04-14）
* [**本次优化和更改了常用插件,请已使用的用户谨慎升级并及时反馈兼容性(常用提示插件理论已经支持到了安卓1.0)**]
* [新增插件] 增加了个数据库插件x-sqlite-s,为Sqlite3的一个操作映射插件,具体见插件目录内的说明文档.
* [新增插件] 增加了人脸检测 xFacedetection，及活体检测。
* [插件优化] xModalS,安卓端弹层修改布局,现在可以覆盖官方的Dialog-page页面了,并且可以接受安卓的返回键及侧滑关闭弹层了.可以覆盖住官方的diaglo-page,做到真正的全覆盖
* [插件优化] xToast在安卓,ios,web布局已更新为宽自动,并设定最大宽(之前宽是固定),方便在平板手机内容超多时自动断行及限定最大宽,安卓端:可以覆盖住官方的diaglo-page,做到真正的全覆盖
* [插件优化] xLoadingS 安卓端现在可以覆盖住官方的diaglo-page,做到真正的全覆盖
* [修复] 修复xToastS事件中的close事件未执行(是我忘记执行了)
* [插件优化] x-vibrate-s现在ios端支持指定震动频率.
* [优化] xCountDown优化,增加了属性captcha,用于验证码倒计时模式,开启后任意地方多个地方使用组件共用一个倒计时值防止重复.
* [修复] x-openweb在ios16+上无法打开外链的问题.
* [修复] x-qrcoder上个版本升级时，定义未删除，导致没有引用模块的人无法打包（有引用的可以打包）。
* [修复] x-crypto-s加密微信端没有同步到其它端。
* [修改] xImageResize微信端没返回文件连接，web改成Base64格式数据，请自己作好条件编译上传。
* [优化] xFloatButton增加了Zindex层级属性。
* [优化] xBetweenTime，对属性quickDate增加一条规则p+x模式表示前x年，方便对数据历史记录的查询
* [优化] x-markdown,增加了对表格的解析及代码的解析,并增加了对表格的样式支持，注意：如果你提供的是html需要自己添加类型样式支持，这里仅是对markdown的解析
* [优化] xDate,xPickerData,xDateView等现支持非正常格式了如：YYYY,YYYY-MM,YYYY-MM-DD,YYYY-MM-DD HH,YYYY-MM-DD HH:mm,YYYY-MM-DD HH:mm:ss
并可通过设置formatSyncValue为true来让format格式化后的格式支持到vmodel中，方便同步表单，xDate库增加了国际化语言设置函数见文档。
* [修复] xUploadMedia组件点击删除时,未触发同步更新的问题.修复,还有可能微信版本会上传两次(不一定存在,但本版本测试不存在了.)
* [修复] 由于hbx sdk4.51+以上造成text嵌套事件失效,导致x-link事件也失效,现更改了内部结构,绕开sdk bug
* [修复] xtabs,项目居中,由于兼容微信造成了app不居中.
* [修复] xTabbar,当设置is-canvas-render为false时,再关闭凸出选项造成背景色丢失.
* [修复] xSticky的change事件忘记携带当前状态了.
* [优化+修复] xImageResizer,新增属性compress压缩值0-1,微信现在开始返回文件地址(之前是base64),web返回base64(之前是Blob链接,会让外层无法上传),并完善了web端跨域出错的提示.
* [优化] xPopover增加了属性maskBgColor,用于控制背景,默认是透明.
* [优化] xTree,增强了parentSelectedFullChildren属性,打开后,现在会根据子节点选中状态,改变当前的三个状态:选中,未选,半选中(仅表示状态,并未选中)
* [优化修复] xbutton,xtag由于微信的版本库本身的bug，导致这两组件里面的文字会没变化，因此本次升级作了兼容处理。（也与官方验证过，确实是微信自身问题）
如果想单独控选中父就要设置为false,具体要见文档
* [优化] x-pull-refresh,组件,由于sdk bug设置了相关一个属性导致安卓机上下拉视觉体验不太好.现已删除恢复到默认状态.
* [优化] x-slider,x-slider-double增加了滑动条上,点击时也可以设置滑块值.
* [优化] xSwitchSlider增加了属性borderColor,borderDarkColor下边线的颜色配置.
* [优化] xSliderMenu添加了属性showToggleMenu,允许侧边菜单展开和收缩为边栏,同时添加插槽toggle用于指定义自己的展开收缩的样式,
* [优化] x-cascader本组件完全重构,增加了属性showCurrentBtn是否显示选中本级,并触发confirm及同步vmodel,最后一项时,也会触发相同事件,同时增加插槽header来完全自定义顶部导航.
list添加了指定icon图标,用于为侧边收缩是显示的图标指示.具体见demo, 同时添加menuPosition菜单位置,left或者right
* [优化] x-scrollx,增加属性showScrollBar,用来是否显示底部指示条.
* [修复] xswiper由于sdk的web 动画缺陷,会导致本组件在切换页面时,丢失相关参数及事件,本版本修复及兼容web相关sdk缺陷.
* [修复] xFloatBottom在web端点击事件失效.
* [优化] xModal,xDrawer增加了btnColor，底部操作按钮的主题色，默认空值取全局，提供时以提供为准。
* [优化] 所有弹层类picker补上disabled属性
* [优化] xtabas增加了click,用于点击时触发,同时修改了change逻辑,同个index时,不再重复触发.
* [优化] Icanvas中的属性rotateCenter增加了"topLeft"|"center"|"topRight"|"bottomLeft"|"bottomRight"等属性类型.
* [cli集成测试] 根目录中jet.test我更新了配置，现在可以测试整个项目了。
* [优化] xEchart图表，更新了opts字符串的解析函数。现在可以解析带函数的opts字符串，且较为复杂的函数
* [优化] xConfig配置中增加了全局统一的xInput的背景色inputBgColor,统一的palceStyle样式配置,方便统一为你的设计稿样式
* [修复] xModal中的按钮色未读取btnColor
* [优化] xSearch增加cancelFontColor,取消的文本颜色.
* [修复] x-image-group兼容了一个hbx sdk的bug可能在部分平台预览顺序不一致.
* [修复] x-popover在设置isClickClose=false失效的情况(也是早期兼容sdk后后期没有更改过来.)
* [优化] x-image,增加了previewSrc,用于预览的图片地址,如果为空,则采用src地址.
## 1.1.10 (2025-03-06)
[优化] 组件文档，已经更新新的解析引擎，文档出错率已经得到降低，为了文档更好，所有组件的相关注释也已更规范的修改+提示
并同时提供离线文档格式：markdown和html两种文件格式，用于大家用AI创建自己的知识库，更好的为你码代码，提高效率。
后期还会提供Pdf离线文档，用于大点的公司内部离线交流查阅组件文档（公派，劳务派遣），保密较高，只有内网查阅。
并同时后期还会提供相关UTS编译技能验证试卷用于认证当前员工是否适合uniappx的开发及是否需要培训的验证。
验证你有没有能力同时开发uniappx并兼容所有端。
[新增] xEdite,富文本编辑器，支持传入markdown,html进行内容编辑（建议遵循微信小程序规范，这样可以适配全部平台）
[优化] 优化x-overflay在微信上的动画表现
[优化] 优化x-slide-verify在微信上嵌套组件内时，可能的错误
[优化] 优化x-echart在微信上事件注册丢失问题
[修复] xToast 安卓端重新设定了动画参数,让打开显示的更自然.同时关闭逻辑改成了:当页面卸载或者超时跳转页面时,不会自动关闭上页的问题.
[修复] xLoadingS 修改了打开动画,显得更自然.同时当页面卸载或者不在当前页面时自动关闭.
[修复] xModalS 修改了打开动画,显得更自然.同时当页面卸载或者不在当前页面时自动关闭.
[修复] xTips 更新了动画,并修复了,如果路径页面,上个页面不会自动关闭的问题.
[修复] xTips,xToast,xLoadingS,xModalS 更新了部分动画,现在不管显示还是关闭都更加的自然
[兼容] x-popver组件在4.51+sdk的安卓上不显示.现在已全部优化,已适配4.51+以上的sdk
[优化] 优化了demo部分页面渲染时机,加快启动速度.
[优化] xTreeFlat组件在安卓上优化了渲染时机
[优化] xNavbar增加了属性maxWidth(仅web生效),height:状态栏高设定.
[优化] xPickerSelected,属性idKey设置为了any,不能为null,现在你的id可以为非null的任意值.
[优化] xBetweenTime,增加了事件dateClick
[优化] xWeekBar,增加了属性emptyValueSelected,默认为true(保持向下兼容),当为false时,modelValue也为空,默认不会选中日期.
[修复] xRequest修复了如果并发时,opts可能没有独立分开,导致可能错误.
[优化兼容] xMarkdown,（建议遵循微信小程序规范，这样可以适配全部平台）预览时增加如下功能:支持文档流式解析文档内容,支持动态高,支持内容复制,本次更新需要大家同时更新hybrid目录内容.
同时这个组件还增加了tagClick事件,用于触发webview内的a,img标签的事件,你可以监听并处理相关逻辑.
同时增加了nodeStyle和nodeDarkStyle请自行配置样式，默认样式我已经清除，为了匹配所有平台。
xMarkdown组件同时从1.1.10开始支持微信(tagClick不支持,只支持预览功能)，同时app,web端删除了编辑功能，请使用全端的x-edite
[优化] xSignBorad组件在demo中增加了弹层内显示的示例.
[优化] xQrcoder，美化了点点，并新增定位颜色单独配置，及新增一个小方格样式，目前四种样式了。
[优化] sign-board,增加了笔触效果，更符合中国式的签名效果。
[修复] xButton对skin的监测。
[优化] xCrpotS加密插件进行了功能升级新增了几个加密函数，具体见demo源码和插件文档
[优化] xInput增加了trim是否自动清空首尾空格(默认为true)
[优化] xCroptS加密插件,修复了微信,web端的base64加密错误,同时新增了sha512,hmacsha512,以及生成rsa密钥对的函数,具体见插件文档.
[优化] xForm增加属性errorAlign用于控制出错时的文本对齐方式,left,center,right
[优化] xForm表单xFormItem的rule校验规则对象增加了一个trigger触发时机选项,默认是change,如果你要设置为blur的话,你一定要是要校验的对象组件是input否则
不触发blur就无法校验.
[优化] xFloatDrawer,在电脑端的滑动体验优化了下..同时逻辑层上已经加入了右滑关闭的逻辑,待官方事件优化即可开启.
[优化] xCell添加了相关属性margin用法同sheet,并在全局中添加了cellMargin,当cell为card时可通过全局修改边距.
[优化] xCalendarMultiple增加了属性rangColor范围选中时的区域间颜色设置(默认为主色的透明0.5)
[优化] xUploadMedia,给用户默认添加的文件对象增加了status,0表示待上传,要确保你的上传文件路径是正确的否则安卓会引发io错误,并且uni.upload是无法捕捉到fail事件中,会导致整个程序不可用
[修复] x-switch-slider 优化并修复相关问题，现在可以让其它在不同端滑动时更加的人性，使用体验更好，页面滚动与左滑不冲突了。（h5不行，因为官方的sdk未设置passive: false）
[优化] x-finger优化了下。
[兼容] sdk4.51后不支持max-width省略在安卓上，帮xCell右宽直接固定了值，如果厉害要宽一点可以通过插槽或者属性配置rightWidth。通过插槽的话，这个属性会失效，需要你自己设置宽
[优化] 现在xImageResiz裁剪，导出的图片全量使用了xBase64file插件，因此裁剪导出后，全平台返回的是图片链接地址（不再分平台有的 filepath，有的是base64了）
[优化] xStepper,新增属性inputStyle可以覆盖输入框样式。新增属性：autoHideBtn，可以自行达到最小值时隐藏减按钮
## 1.1.9 (2025-02-16)
* **非常重要**改动较大,如果遇到小问题,请自行提交到coding上,按规范提,不要一直@我,发布大版本,有点小问题属正常.后续更新修正即可.不想升级的保持在1.1.8即可.
* 【非常重要】本次更新非常大，大多数插件对微信了适配，比如图片识别码也能在微信上实现适配（非微信官方弹摄像头，而是直接本地图片识别），更新力度非常大
* 带来的4端适配也是非常好的，本次更新耗费了我大量的时间及精力（适配前后花了约2个月，适配微信），因此本次升级后授权费用涨价50元，由之前的499涨至559元
* [新增组件]思维导图 xTreeFlat，这是一个高性能的即时渲染思维导图组件原生绘制，无惧性能
* [新增渲染库]ICanvas.uts，是一个强劲图形渲染库,原生绘制，无惧性能，事件系统独立每个元素都有自己的事件，还有独立的动画系统。
* [重要]x-qr-u，已经删除，请使用x-qrcoder代替，x-qroder已经完成纯原生绘制，因此不再需要依赖第三方插件。
* [重要]由于插件和组件(部分)改动较大,如果升级后出现编译错误的,请定位问题,查看文档和比对源码逐一修改,有些修改日志可能没有充分体现.
* [适配微信] 这里着重讲下差异 **如何适配到微信**
* 1. 默认的插槽如果含有插插槽数据的,在微信端直接在标签内写内容是会被直接忽略不显示正确的做法如下:
```vue
<template v-slot:default="xx">
 正常的标签内容
</template>
```
因此我大部分默认的插槽数据会被删除,以方便大家直接兼容微信,而不必写上面的格式,但缺点是数据没了.但相对大家可以一步过渡到微信,只选伤害为次的方案.
当然有的组件可能我判断用的少,而不是大量使用的组件我会保留,那么你使用了,导致不显示了则需要如上格式写全才能显示.
* 1. text直接内部插槽会让内容失去响应，就是改变数据，页面不更新，解决方案是给text套view再写插槽，
* 影响我的xtext组件（已经作了条件编译，这里是提醒你们注意兼容sdk)
* 2. 有些组件为了兼容可能推了一些屎山代码,影响微信的性能,但不多,整体以兼容为主.
* 3. app专用的原生插件大部分无法在微信上使用,请自行做好兼容,做条件编译.
* 4. 小程序不支持动态插槽，只支持同名或者不同名的循环插槽，这意味着一些需要通过动态插槽来布局的组件将失效，仅支持内部默认样式
* 已知组件：x-table
* 5. 小程序不支持元素大小响应监测，只支持query page监测，因此组件内无法预测组件相对窗口的大小变动，因此无法支持UniResizeObserver，所以
* 你在app,web上组件能响应式，但在微信上无法响应式。
* 6. 插槽内，如果不套个根节点，而是直接循环多节点，微信小程序内会自动给插槽套个父节点View，导致布局异常，请注意sdk变化。
* 7. 前期已经做好的项目升级兼容微信,基本改动很少,就可直接无缝发布到微信小程序
* [重要]x-drage,由于兼容微信的原因事件touchstart,touchend,touchmove改为start,end,move,请注意兼容
* [优化]x-modal-s,优化三星S21-S23机型安卓端点击遮罩层不关闭的问题
* [优化]x-tabar可能存在0.5像素的差异.
* [优化]x-weekbar，修改为无限滑动切换周次。
* [优化]x-msg-notice,可以允许向上,向下,向左,向右的手势来关闭消息通知栏.
* [优化]瀑布流组件,demo示例我已经已更新请大家参考demo布局,以实现高性能无限加载体验(另:安卓已经采取多线程异步处理数据显示提高单个瞬间加载显示的流畅度)
## 1.1.8 (2025-01-08)
**祝各位2025/蛇 新春快乐**
* 建议使用sdk 4.36,4.44,4.45来使用本版本
* 已知sdk4.44+BUG: ios动效部分不流畅,安卓数字自动加.0(老bug复发)
* [新增]剪切板操作插件 x-clipboard-s
* [新增]插件x-animation-s,原生动画,可增强交互体验,可以实现3d等复杂动画(比如3d翻转等营销交互)
* [新增]插件x-sse-s,流数据读取.使用方法见代码仓库
* [重要更新]xStore新增了个函数setConfig(config as utsjson)可以统一设置全局所有属性,所有字段为可选
同时xConfig中增加了属性字段为theme:new Map<string,string>(['primary','#ff0000'])此字段可以覆盖内置所有同名主题的颜色,比如默认的primse,success,error等常用的主题颜色名称
* [优化]x-between-time,修改了样式细节,修复了上个版本导致标题塌陷.
* [修复]x-checkbox,印象中是从1.1.4开始改了非选中值为空值,demo和内部未同步,导致半选状态失效.
* [优化]xConfig,增加了全局属性closeIcon,不同人的设计稿可能对右关闭图标样式有要求,有需要线性有的需要面性图标,现在统一全局更改
* [优化]x-float-drawer开启滚动时的视差体验，目前可以与头条的体验相仿。
* [优化兼容]x-image-resizer兼容sdk 4.44安卓端,这个可能还后期如何官方修复了sdk bug可能还要再改，但目前在4.44安卓是正常了。
* [修复]x-col的精度在安卓上可能导致除不尽的列数出现断行。
* [修复]x-icon，可能旋转不在中心点上。
* [优化]x-search添加属性darkColor用于自定义暗黑背景。
* [优化]x-switch-slider,补充了些事件,demo增加了个列表循环手动状态管理的示例
* [修复]x-slider,x-slider-double,在设置max及step时,精度问题导致的一些问题.
* [修复]x-mlkit-scannig-s的web端函数名称错误
* [优化]x-drag,增加了touchstart,touchmove,touchend事件,属性:scrollDiff滚动距离,touchmove是在页面需要自动滚动时触发并且间隔200ms,
具体见demo,demo中增加了:自动滚动,局部拖动区域的演示,方便大家学习参考.得益于sdk4.36+起ios sdk已经能阻止页面滚动的冒泡,现在已经放开了触摸组件时允许滚动页面.
* [优化]x-dropdown-menu，增加了切换动画。

## 1.1.7 (2024-12-24)
* [重要]仅支持正式版本4.36+(alpha4.37+)运行,(不可使用alpha4.34,35,36)
* [新增]x-base642file-s插件,用于将图片base64数据转换为本地图片文件对象,三端兼容,web转换为Blob对象文件,ios,安卓为缓存文件路径.
* [新增]x-image-resizer组件,用于将图片裁剪为指定的比例尺寸图片,三端可用,可双指缩放操作.(安卓在4.36上暂无法截图使用,sdk未知bug,待官方沟通修复)
* [新增]x-color-view,新增一个用户界面操作友好,精致的颜色选择器,多模式选择,为你的app增彩.
* [新增]x-x-translate-s,谷歌大模型翻译,预下载后可以离线翻译.
* [新增]x-runnable-s，安卓，web端可用（ios官方限制不可用）。多线程执行插件，主要用于复杂，数据处理在不影响页面流畅度的情况下在后台开线程执行逻辑。为后续xCanvas库提供强劲动力。
* [新增]x-screenshot-s插件,可用于对应用页面进行保存,从而分享页面,对局部节点保存并分享,(分享需要使用我的另一个分享插件)
* [新增]x-zip-s,解压缩插件，主要用于比如对沙盒内的一些缓存，临时数据资料打包后统一上传，或者从服务器下载压缩配置文件统一解压到应用依赖。
* [新增]在1.1.7开始新增新的标准组件库依赖x-design,里面的组件开头为xd-xxx,从这版本开始凡是以xd-xx开头的标签统一为原生标准组件需要打包,或者搭配环境使用.
* [新增]xd-blur 模糊标准组件见x-design下的readme,主要功能:模糊标签内的内容本身,模糊它底下的背景(即磨砂背景效果),功能强大,可以应用模糊任意内部内容和任意它的父级(背景)
* [增强]重构了插件x-crypto-s,支持了大量的加密算法见文档,另外sm2,sm3,sm4本次未更新见下个版本更新.
* [增强]x-upload-media,早期因为官方不支持视频选择,现在官方支持了,因此我也扩展了下.现在可以上传视频或者图片,参数:mode:video,photo,及视频参数videoOps,具体见文档.
* [增强]x-camrea-u,相机增强了start,stop,pause视频录制功能,增加事件recoder,具体见demo,可以在录制视频的同时进行抓拍相片.停止或者拍照时现在不会关闭相机,需要你手动关闭,方便连续拍照
* 如果你想实现点击拍照,长按录制请根据提供的方法,结合套个view通过事件执行方法实现.
* [优化]x-tabbar因为hbx sdk的对canvas渲染的不稳定性，我提供了两个属性：firstRenderTimeout，isCanvasRender用于回退使用。请自行选择用哪个引擎渲染界面。
* [修复]image增加的插槽,不小心vif写错位置了.
* [优化]radio增加了onlyChecked属性,,出现这个属性时,group或者单个使用时将不允许取消选中,至少保留一项选择.
* [优化]x-radui-button增加了项目中disabled,用于控制某菜单是否要被禁用.同时增加了按钮click事件(不管是否禁用都会触发)
* [修复兼容]x-drawer动画在h5会被执行多次，造成事件被执行多次，由于这是底层事件各平台对齐原因。现把三平台对齐只执行一次。
* [修复]x-input的clearColor颜色忘记使用了。
* [优化]x-button删除_style,_class,没有意义的属性，可以直接在组件上写style,class.
* [兼容]x-tabbar兼容了4.36+
* [修复]lines在web端如果动态设置0时,不会生效.不清楚是sdk bug还是web规范.现在已经兼容修复了.
* [修复]x-picker-time时间不准确的问题.
* [修复]x-action-menu,修改跳转时机,避免触发sdk 底层渲染bug
* [优化]x-pull-refresh 增加了disabledPull,disabledBottom禁用下拉刷新和触底刷新.增加ref方法:setScrollTop,setScrollIntoView
* [优化]x-markdown背景整成了透明.
* [修复]x-float-drawer内容无法滚动的问题.
* [优化]x-float-bottom增加了thresholdBottom,thresholdTop底部和顶部的安全距离的单独控制。
* [优化修复]x-image组件,由于sdk image组件自带bug造成了我这组件在ios容易丢失图片,本次更新兼容了sdk bug,ios端不会再丢失图片了.
* [优化]x-slide-verify兼容pc操作.
* [修复]x-input,clear,与右图标上下不对齐的问题
* [优化]插件x-qr-u,增加logo设置,增加ref方法getQrcodePath,用于获取码的图片路径(web返回的是Bolb文件,安卓,ios返回的是缓存图片路径).
* [优化]x-drawer，增加了属性：offsetBottom，用于对底部的偏移，有些情况可能需要在带tabbar页面弹出。
* [优化]x-swiper,修改了反转播放逻辑,现在播放到最后一个时,直接设置为0从新播放,不再倒放.
* [修复]x-echart,不支持函数参数格式化,更新时,请大家重新复制hybrid到你项目中.
* [优化]x-icon,新增fontFamily，code属性用于自定义图标，当提供code字符时以code图标字符为准，name将失效。fontFamily可以定义为自己的图标字体，需要你自己先安装好字体
* [优化]x-ocr-s,ios,安卓添加本地路径识别函数localFilePathImageBuilder,可自己循环批量处理.
* [优化]x-inputtag,增加maxCount限制,及修复设定高时,不顶对齐.
* [优化]x-slider-menu,标题设置最小行高
* [优化]x-overlay增加了参数disabledAni控制禁用点击时,是否开启弹跳动画,默认开启.
* [优化]x-cell,提供了padding属性,可以自己改变内边跑,当然你也可以不用,直接在组件上写style是一样的.用法与sheet一样,并且修改默认值让card与非card间距等同.
* [优化]x-dropmenu组件优化了布局以应对更复杂的嵌套应用,增加了参数:zIndex(嵌套时需要用到层级),hidnMask(隐藏遮罩,试图在全局遮罩中嵌套时web端需要隐藏)
* [优化]x-float-drawer,增加了参数contentMargin,用于控制内容层的边跑.
* [优化]x-cehckbox,x-radio,添加属性:default-checked,只接受初始值,默认是否选中,不会影响选中的值,只影响选中状态.在group中不会起作用(与多个大厂比对过,确实合理,不然值会冲突)
* [优化]x-picker-selected,增加:disabledPull,disabledBottom,v-model:refresh,v-model:bottomR-refresh，增加事件：refresh，bottomRefresh，用于支持搜索数据分页加载，触底和下拉刷新
* [优化] **x-tree更新如下:**
2. floaderIcon自定前缀打开和关闭图标/图片
3. showFloaderIcon显示或者隐藏上述
4. showChecked显示或者隐藏选择框
5. checkedIcon选中和未选中框图标/图片
item项目中增加字段:
1. showEdite用于控制是否编辑节点标题内容
2. showRemove显示删除按钮
3. showAdd显示添加子级功能
* [优化]x-more,增加unActiveColor,及自行判断要不要显示展开按钮:showMoreBtn
* [优化]indexbar,右侧字号改的更小,尺寸也更小了,以方便在更小尺寸上展示防止裁剪.
## 1.1.6 (2024-11-21)
* 仅限hbx sdk 4.32+使用
* [适配]sign-board 适配4.34sdk 使用
* [适配]x-echart图表 ios端在4.33+无效.现适配了4.34,可用了.ios端的格式类型与安卓不一样.一个是UTSJOSN,一个Object.
* [适配]x-markdown,ios端适配了4.33+
* [修复]between格式化写反了导致前两个快捷选错了。
* [修复]x-tabbar修复兼容4.31+ sdk bug
* [更新]图标库更新到4.5.0
* [修复]主题修复兼容4.31+ sdk bug
* [修复]x-ocr-s ,ios没对齐安卓,失败不会返回回调.
* [调整]调整x-keyboard-number位置，及补充车牌键盘字符。
* [调整]x-tabbar不同平台的兼容性，同时提供全局tabbar高的读取，xTabbarConfig.tabaarHeight
* [新增]新增x-mention提及组件,用于聊天或者需要提及符输入时有用.
* [修复]x-between-time如果切换过快会造成数值可能没有回正为正确值.
* [优化]x-float-drawer补充了beforeClose事件,另外增加了movestart,moveend,以及heightChange百分比.配合这些参数及dom操作可以满足达到头条那样的丝滑的反差差值缩放视频和评论大小的关系动画.
* [优化]x-table增加属性:refresh异步函数刷新时执行.增加事件@refresh刷新时触发.
* [优化]安卓端的扫码插件添加了几种常见条码识别类型，ios是全部类型，安卓因为有些机子性能差，不能全部加，不然识别慢。
* [优化]x-mlkit-scinnig-u:新增属性flash默认是否开启闪光灯.后续如果需要动态打开 和 关闭闪光请使用ref方法:setFlashModel(boolean)
* [优化]x-mlkit-scinnig-s界面中底部操作按钮中间增加了打开/关闭闪光灯的按钮,以便夜晚识别码的需求.
* [优化]x-button添加navigateMode,用于指定跳转类型.
* [优化]x-image,增加error,loading两个插槽,方便自行定义加载和错误的样式.
* [优化]x-picker-selected增加了插槽名item,并携带了数据名v-slot:item="{item}",类型是UTSJSON,并同时增加了属性itemHeight,用自定义自己设计排版项目样式.
* [优化增强]x-pull-refresh增加事件@scrollDirection和@scroll，前者是滚动方向改变时触发，后者是滚动时触发。
* [优化]x-float-drawer增加zIndex层级设置
* [优化]x-tabbar增加zIndex层级设置,增加list，增加activeIndex用于单独控制，不依赖全局，当采用本地值时，需要自行控制索引选中
* [修复]x-tabbar当暗黑切换时，没有重新渲染。
* [优化]picker系列组件：picker-city,-date,-selected,-time，-modal，-overlay等增加了zIndex层级属性设置默认全是1100
* [优化增强]x-slider-menu改动较大,但保持 了 向下兼容:左侧菜单栏目改成了虚拟列表提高性能.增加了layoutMode=default兼容旧版本,layoutMode=scroll时会开启右边循环左侧菜单
* 栏目的项目节点作为定位节点,会开启右侧滚动左侧菜单定位,左侧点击菜单时,右边内容自动定位滚动的效果.增加了itemSelectedStyle用来覆盖菜单项目的选中的样式,可以定义自己样式.增加了itemHeight控制菜单项目的高.
* 建议参考我的demo使用.
* [优化增强]x-slider-tree子分类全采用list-view虚拟列表.
* [优化]x-table增加了showScrollbar属性.同时隐藏了pc版本时多条的滚动条.
* [优化]所有picker弹层系列增加了cancelText,confirmText字段用于控制多语言文本或者定义自己的文本.
* 
## 1.1.5 (2024-10-27)
* [重要更新!!!]x-form的rule加强了校验,由于你们可能需要让值为null,这牵涉到联合类型,因此你们的校验函数vaild(val:any)=>boolean要改成valid(val:any|null)=>{}
* 同时修改了FORM_RULE类型唯一影响的就是vaild你们要适配下,这个类型的所有参数改了可选,并且增加了max,min来快速校验值,同时增加了指定type类型校验:这对你们快速校验非常有用它增加了对:date,array,email,phone等快速验证.
* [优化]x-tabbar属性outIndex从此版本开始不作限制,可以任意索引凸起,并且增加了属性可以控制是否动态镂空,
* 而且还可以响应式尺寸镂空.通过static下的遮罩可以更改凹陷的造型.同时也可以使用背景图.此版本更新后样式个性极为灵活.（注：sdk 4.31 ios官方有bug连带这个特性在ios上有差异，请后续关注官方更新）
* [修复]xUploadFileMedia在ios端放开压缩后产生类型丢失,造成无法上传.
* [修复]日历相关相组件在app端绘制dot时位置错误.
* [修复]x-drag可能长按不移动时造成位置混乱.
* [轮播]指示点可能让底部事件失效。
* [修复]x-picker默认首次为空时默认第一项确认可能在安卓平台造成无法选中默认的第一项值.
* [增强]x-input-tag添加Postion属性用于输入标签在输入框内还是在外两种排版布局.并优化样式.
* [优化]x-search增加placeholderStyle属性用于控制输入框提示样式.
* [修复]xtabbar由于sdk中的computed 无法动态返回store的对象值造成内容改变时无法监测变动.因此换了个写法兼容sdk的问题.
* [修复]x-code-input双向绑定失败
* [修复]x-switch-slider在弹层内失效(由于节点被传递到外层导致内部无法查询节点.)
* [优化]x-btweent-tiem上方的快捷按钮设置为带选中状态,点击时高亮选中.滑动时间时会自动丢失选中状态.
* [修复]x-btweent-tiem上方的快捷按钮如果设置为空时,间隙丢失了.
* [优化]picker系列组件增加了modelStrJoin用于modelStr拼接时修改拼接符号.
* [优化]x-markdown增加了属性isHtml用于纯html渲染，直接内部赋值html,以避免特殊字符在安卓上触发sdk bug
* [优化]radio，checkbox增加了labelSpace间隙属性用于自由布局时，好控制间隙。添加了cell布局示例。
* [优化]x-tabs改变了移动方式,现在改为如果内容能滚动,选中的内容始终处于组件居中的位置.
* 同时废除了moveCount属性,增加了isItemCenter属性用于项目少时,想居中(配合前述可自动居中),增加了line-gradient用于设置渐变线条.
* [优化]picker,picker-view添加了unitsFontSize用于修改单位字号大小.
* [优化]x-wiper-item组件增加了round属性,用于单独控制圆角.
* [优化]x-qr-u,根据谷歌官方安卓端升级了码库为3.5.3(最新2024-1),并调整了配置减少内存占用,并且生成码时在多线程内执行防止UI阻塞
* [优化]x-waterfall瀑布流组件,采取了虚拟节点渲染,使用时请务必让组件在scrollview节点内(安卓和ios,必须要让组件处于滚动中可以是listview),web没有要求.web是自带的滚动处理虚拟节点.
* 本次更新后:安卓需要打包后才能测试出性能比之前提高了非常多的性能(至少提升100%),ios端提升也是非常明显(与web提升一样).web端提升更多(基本可以无限的下拉不限数量,也不卡再多也不怕.)
* [优化]x-between-time，添加了属性disabledClear，是否允许用户清空选中的数据。如果开启，那用户清空了选项后无法按确认键，也就是无法让数据为空。
* [新增]x-svg-u插件，用于显示svg文件。三端通用。请务必4.31打包。
* [优化]x-camrea-u,增加属性orientation以控制前置,后置摄像头,flash 闪光灯是否开启,web不支持这些属性
* [重要,不向下兼容]x-ocr-s升级了调用方式,使得安卓,ios用同样的方式调用,不再区别,统一使用callback,并且在回调中携带回了坐标,以便让大家通过坐标计算识别比例或者绘制位置.并且文本块统一为行返回(之前是文本块返回,但在源始数据中还是块和坐标)使用方法见文档或者demo
## 1.1.4（2024-09-23）
* [优化]picker-date,date-view默认不同步格式化后的值(如果为空时)
* [修复]x-waterfall组件上个版本为了实现响应式瀑布流，可能导致ios端异常闪退。
* [增强]x-waterfall增加isResize属性用于关闭和开启响应监测能力。关闭后不会跟随尺寸响应。
* [新增插件]x-svg-u,用于显示svg图片或者图标.
* [修复]markdown动态渲染,追回内容时不会渲染.
* [修复]upload-file在ios被上传时会被执行两次.
* [优化]x-table标题和单元格的style中增加一个align属性,用于控制元素对齐方式(用自定插槽也可以自己实现并完全自定)flex-start,center,flex-end
* [优化]input增加focusBorder及全局配置增加inputFocusBorder来控制输入框聚集时候的边框样式来显示高亮.
* [修复]pull-refresh组件如果model=listview时内容没有切换为list-item相关.导致长列表性能差.切换为这个模式的时候你们的布局子项必须为list-item
## 1.1.3 (2024-09-15)
* **重要:本版本仅支持hbx sdk 4.27(群测版本或者4.27+) 0910,以下的sdk版本不支持使用.**
* [优化]x-drawer,适配无安全区域时的间隙（主要是web).
* [修复]file-s插件由于安卓(sdk 机制 原因)共享进程的原因,导致file-s结果集可能始终在上次页面中.导致离开当前页面后,再进来选中的结果执行不在当前进程内.
* [修复]date-view,picker-date初始值设置时可能与选中值不符,经format值也与初始值不符.
* [优化]x-image不再采用接口getImageInfo返回的缓存地址,由于官方迟迟无法修复bug,故采用回原始路径(可能会造成缓存与实际加载时触发两次加载,具体未测试.)
* [修复]uvuePx.min.css压缩工具压缩后丢失了条件编译。
* [优化]button增加了fontWeight属性,默认normal
* [优化]x-tabs添加了动态插槽,用于自行定制自己的标签样式.
* [优化]x-taost-s修改了ios上下对齐样式.兼容4.2.7
* [优化]x-loading-s兼容4.2.7
* [修复]x-mlkit-scannig-s ios异常。
* [修复]x-file-s在安卓端重复创建实例.
* [修复和优化]x-tabs增加属性moveCount，用来给你自行控制选中时的位置偏移个数，以满足你们需要选中项居中的需求，同时修复可能数据超多时，会被遮挡的问题。
* [修复]x-switch在ios端如果出现在弹层内可能异常的问题（sdk底层导致）
* [优化]x-runstart-logo,ios端将弹层注入到了app根节点.这样不管跳转到哪个页面弹层始终在最前面.
* [新增插件]增加消息提示插件x-tips-s
* [兼容]x-mqtt-s对最新的4.27进行了回调适配
* [兼容]x-network-change对最新的4.27进行了回调适配
* [更新]echart.min.js更新到了5.5.1请注意更新并复制hybrid目录到你的目录中。
* [更新]图标库已经更新到remixicon 4.3.0
* [试验]x-image增加属性lazy用于在scorllview根节点的页面进行懒加载,不可视范围内的不显示.请仅慎使用,不可在list-view中使用.
* [优化]键盘系列组件增加了属性hold用于是否保持键盘不收起.
* [优化]为了切实解决原生节点过多造成drawer阻塞动画问题,我增加了个lazy属性用于内容懒加载,参考大厂的做法,动画界面优先,内容延后展示.此属性默认是关闭的.如果你节点内容复杂建议开启.
* [优化]x的成熟,我已经放开了form,radioGroup,checkGroup内仅允许它的直接子节点,现在可以在这些父节点内随意嵌套其它组件进行更自由的布局.但不建议嵌套太深,影响性能.
* [优化]x-float-button在web端增加事件冒泡阻止.
## 1.1.2 (2024-09-11)
* **重要:本版本仅支持hbx sdk 4.27(群测版本或者4.27+) 0910,以下的sdk版本不支持使用.**
* [优化]x-drawer,适配无安全区域时的间隙（主要是web).
* [修复]file-s插件由于安卓(sdk 机制 原因)共享进程的原因,导致file-s结果集可能始终在上次页面中.导致离开当前页面后,再进来选中的结果执行不在当前进程内.
* [修复]date-view,picker-date初始值设置时可能与选中值不符,经format值也与初始值不符.
* [优化]x-image不再采用接口getImageInfo返回的缓存地址,由于官方迟迟无法修复bug,故采用回原始路径(可能会造成缓存与实际加载时触发两次加载,具体未测试.)
* [修复]uvuePx.min.css压缩工具压缩后丢失了条件编译。
* [优化]button增加了fontWeight属性,默认normal
* [优化]x-tabs添加了动态插槽,用于自行定制自己的标签样式.
* [优化]x-taost-s修改了ios上下对齐样式.兼容4.2.7
* [优化]x-loading-s兼容4.2.7
* [修复]x-mlkit-scannig-s ios异常。
* [修复]x-file-s在安卓端重复创建实例.
## 1.1.1（2024-09-04）
* 【请务必使用hbx sdk4.26+ 使用本组件库，不能低于此版本，否则部分组件异常】
* 【4.26sdk bug预警】我已经提交给官方修复：[issue](https://issues.dcloud.net.cn/pages/issues/detail?id=7832),会导致我的轮播套了图片后失效。解决办法是套了image,或者x-image的
* 需要在标签上写style="pointer-events:none"来屏蔽Image上的事件，通过swiperClick事件来捕捉。
* **非常重要**源码用户和加密用户在main.uts中的引用安装方法已经改了,请使用4.25+的用户立即查看文档修改
* ```原源码用户:import {xui} from "@/uni_modules/tmx-ui" 需要改成 import {xui} from "@/uni_modules/tmx-ui/index.uts"```
* radio未选中值改成了''
* [重要修改]x-camera-s扫码插件, openCameraApi接口调用增加了第二个onlyCamre是否仅使用相机识别,不通过相册识别.请使用时传递此参数做好向下兼容,不传第二参数调用会报错.
* [修复]x-picker-item组件在复杂的嵌套弹层节点深时,宽会丢失原因不明,目前已经固定了宽.
* x-checkbox-group类型标识错误
* x-navbar静态时黑夜设置为透明时，首次不透明。
* x-radio,checkbox使用了安卓sdk不能使用的===导致可能选不中。
* x-text正则有问题,现已修复
* x-upload-media为了解决手势冲突问题,排序改回长按启动排序功能,一是为了防止手势冲突,二是解决容易误解拖动排序.
* x-slider-double,x-slider不知道从哪个版本的sdk开始就不行了,现已在4.23中适配
* dropdown-menu优化背景遮罩位置设计在菜单下方,当菜单想要透明时,可以避免遮罩在菜单底部.
* [新增]金额栅格 xMoney组件，用于金额格式化和显示，完整的保持精度避免3.99 = 4的情况。
* [新增插件]x-openweb,打开外链(使用默认浏览器打开)
* [新增组件]x-link可以打开应用页面或者外部链接.
* x-share-s插件解决uniapp项目无法在安卓上分享本地图片资源问题，同时兼容uniapp ios分享的问题。
* [新增插件]x-call-phone,拨打电话
* [新增插件]x-runstart-logo,用于在appvue中直接显示该插件以展示启动logo图,可以自己定义背景颜色,logo图,版权声明等属性,并配合我的xModalS以达到隐藏协议声明前禁止访问应用的布局.
* x-keyboard-number新增digit用于是否开启小数键盘或者整数键盘。
* x-navbar改成了setup模式组件。
* x-loading增加了hideText属性，用于隐藏文本。
* [优化]waterfall瀑布流增加了响应式处理，可以响应屏幕尺寸，如果你的图片需要高响应，建议你用rpx单位。
* [新增组件]x-slide-verify,滑块验证.
* [升级插件]x-mlkit-scannig-s安卓和ios升级为全屏扫码,ios支持相册选择图片识别.并兼容安卓5.0+
* [优化]扫码插件相册的识别率,升级ios扫码也支持相册 识别 码,至此三端安卓,ios,web已对齐.
* [新增插件]x-camrea-u相机组件,用于实时捕捉画面(止前功能相对简单,但至少可用),三端支持
* [修复]x-tabs中的lineColor不生效的问题.
* [新增插件]x-crypto-s 常用加密插件,三端可用.
* [修复]x-steps vmodel变量无效的问题.
* [优化]x-steps 新增属性reverse,用于状态反向展示.如果你先前了竖向的,要用回之前的倒序你需要设置reverse为true,请做好兼容.
* [优化]x-collapse现在开始已经支持了自适应高.
* [优化]x-navbar新增属性llWidth,来控制左边宽.之前是lrWidth控制两边宽的,以达到居中效果.
* [新增组件]x-weekbar,时间周组件,样式丰富,非常精美,能够适应不同设计要求.
* [优化]x-more 现在支持响应式,异步加载内容,自应内容高动画.
* [优化]x-keyboard-number键盘新增mode，number,password模式，可以以纯数字符串形式当作密码用不会限。
* [优化]x-calendar-view增加属性：hideHeader，disabledSwiper，disabled，同时增加了切换日历的方向：左右或者上下（无限循环）。
* [新增组件]x-alert 警告提示框,样特别丰富且非常精美。
* [新增插件]x-network-change 用于监视网络状态变化,及时知道当前是否有网络连接.
* [更新]x-input新增同步官方的属性inputmode,holdKeyboard
* [优化]x-snackbar兼容响应式尺寸显示.并修改动画,视觉效果更为快速流畅,同时postion开放了6个位置,具体见文档.
* [优化]x-popover 修改动画,视觉效果更为快速流畅
* [优化]x-drawer增加了offsetTop用于偏移顶部的偏移量，zIndex用于弹层的层级。
* [优化]x-loading-s，现在把最小高100设置成了config.size，因此你可以设置弹层小点，但你同步得把iconSize弄小，不然不会正方形，因为高是最小高。
* [新增库] xTween库增强动画,试验性质必须4.26+ sdk版本
* 【修改优化】x-animation采用xTween库制作动画,删除了属性revert，因为name已经有成对的动画，可自行修改name反向播放。并且动画已由css动画改成了刷新回调动画。
* [优化] x-overlay 采用xTween库增强动画,之前这个组件是没有动画的.这是一个试验性质,如果出现问题,请及时联系.
* [兼容] x-barcode兼容4.26+ sdk,并修改了样式,文本底部对齐线条两端.
* [优化] x-image添加了模拟的click事件，原生的click事件已抛弃，原因在于原生 的click有移动事件中容易误触发。
* [优化] x-modal-s修改了代码，兼容uniapp应用。
* [优化] x-icon因为css动画的不可控导致一些需要旋转时部分机型异常。现在已经完全用xTween插件替代了动画，应该不会有异常了。
* [修复] x-image，属性mode写错了。
* [修复] 颜色对rgb的校验错误，导致不能设置rgb颜色。
* [视觉优化]x-button和x-input把圆角默认统一改成了12,同时主题配置中给Input配置了统一修改圆角的属性,方便修改默认全局的输入表单圆角.
* [视觉优化] x-cell之前改造时rpx没改成默认的px样式不统一,并且crad时,左右间隙改为了14相比增加2px
* [优化] 优化了主题颜色,修改一些组件的默认样式及动画细节,使得组件更为精致.
* [优化]x-tags增加对skin属性的动态响应。
* [优化]x-upload-file增加上传执行的函数beforeUpload,用过上传前的过滤或者验证类的需求场景.
* [优化]x-form-item增加showError属性用来控制和隐藏底部的错误提示。
* [优化]x-dropdown-item增加render属性,用于绕开一些组件触发sdk bug,具体使用说明见文档注解.如果你没有问题不用理会本属性.
* [优化]x-image-group组件因官方的函数uni.previewImage有bug导致预览时索引异常.
* [优化]x-file-s增加了filter类型过滤器,对应x-upload-file也增加了相应的filter,但限制和注意事项请看文档使用,这是一个权衡后的属性有限制和注意事项,请着情使用.
* x-upload-file增加了beforeUpload用于手动文件过滤,x-files-s,三端拉齐了一些函数,都需要执行remove,clear函数来清除和移除某文件.
* [修复]x-loading横向向没对齐
* [修复]x-tabbar项目选中中设置disabled没起效
* [新增组件]x-avatar-group，头像组，设置丰富，可以文本，图片，占位
* [新增插件]x-mqtt-s，用于连接webstock，ws,wss,订阅和推送消息（ios wss/tls需要证书）
* [优化修复]x-setps组件竖向可能的异常。并修改了相关视觉效果。
* [优化]x-radio-button增加了响应式布局,优化了圆角细节。
* [新增插件]x-window-grey,让当前页面变灰,如果想任意页面变灰,可以通过minx,然后在onShow中执行变灰操作.
* [优化]x-pull-refresh,由于listview,scrollview在最近的版本中ios已经问题不大因此开放了下拉刷新自定内容,同时新增了mode用于内部采用listview还是scrollview来渲染.
* [插件增强]x-camrea-u组件新增ref函数chooseImage,相册选取识别码.
* [优化]x-upload-media增加属性beforeComplete，用于每个图片成功前修改类型和结果值，达到人为判断是没权限还是什么让上传成功或者失败。
* [优化]x-slider及x-slider-double值的转化为1为单位，过滤掉小数
* [优化]x-table增加了rippleColor，和rowCellColor等间隔背景及非间隔的默认行背景。
* [新增库]xCavas预览版本
* [修复]x-drag最后一位横向移动时可能异常.
## 1.1.0（2024-08-1）
* x-mlkit-scannig-s升级支持IOS12+扫码,WEB PC、H5实时扫码识别(无扫码点仅单个识别)
* x-mlkit-scannig-u升级支持IOS12+扫码嵌入式页面,同时支持WEB PC、H5实时扫码识别(无扫码点仅单个识别)
* x-qr-u 更新已经支持了web/h5,已经兼容了三端，想要快速的生成码，请使用本组件，而非webview版本的x-qrcoder。
* 【为了你能支持web,h5支持实时扫码】请同步更新static目录js文件。
* **重要： checkbox,checkboxGroup可能会影响你们的业务，id修改成了string类型**,另外js中""==0=="0"==false，因此你们的字符id不要以0开始，因为安卓不支持===
*  删除了插件 toast,loading 的原生行高属性，以兼容低版本安卓。
*  x-float-drawer，删除了 preventDefault 属性以 免部分安卓机丢失事件
*  x-picker-selected 在安卓端加大了 lazy 的延迟量,并增加 lazy-duration，设置为 100ms.用于部分低性能手机可能效果不明显的问题。
*  x-swiper，事件冲突问题修复
*  x-picker-date，默认值校验问题修复
*  x-search，改善了动画视觉效果。在安卓由于input的sdk vmodel 5ms vue模型数据响应的问题会导致安卓丢字符，现已修改赋值方式绕开这个vue模型，解决安卓丢字符问题。
*  picker,picker-city全部放开了默认文本回显示，由于要大量递归计算回显，表单过多，时请自行考虑性能问题。
*  修复switch-slider可能的事件丢失与手势冲突。
*  x-navbar现在支持了动态悬浮时的动态渐变背景切换，为了实现此功能，布局已做了修改已兼容安卓。同时增加staticTransparent用于指示在静态顶部时，是否透明背景。请各位做好兼容修改。
*  控制背景时变得更强大。
*  x-request,类型拼写错误更正，请大家更新时如何报错记得字母内更新下加个q
*  code-input，新增了place-shape属性用于组件内占位闪烁的动画效果，空值可去掉。
*  xDate增加了个函数方法fromBetweenLongTime用于返回离指定时间多久，比如：刚刚，1分钟前。。类推
*  xDate增加了个函数方法：diffTime，与目标时间相差多少个单位比如两时间内共多少天，多少小时之类的。
*  swiper增加showDot属性，同时修改了一些其它问题。
*  新增组件 x-date-view,使用方法与 x-picker-date一样，一个是内嵌的一个是弹层式的。
*  xDate新增方法getQuarter，用于获取本年或者本时间所在的季度。
*  x-calendar-view日历组件修改了视觉效果，并兼容了宽屏和响应式屏幕。现在app端也可横屏竖屏都完美了。
*  新增组件x-calendar-multiple日历组件,支持多选和范围选择，支持限定选择的天数。
*  修改了xColorUtil下的暗黑计算，影响了主题为thin的视觉效果。
*  x-picker-selected增加了list监听回调。
*  checkbox,checkboxGroup可能会影响你们的业务，我调整为字符串了。之前是number，主要考虑到id多以字符为主，而且数字没有实际意义，字符比较好命名。由于不支持联合类型，暂为字符。请及时修改
* ximage组件改写了加载逻辑使用了sdk最新的api修改以适应原本的sdk宽高的缺陷。（仅安卓和web有效，ios还是以前的）
* 修复x-loading-s,x-modal-s,x-toast-s在低端安卓几下不能关闭的问题，目前已经调至可支持到安卓4.2+版本（含），x-loading-s新增了size属性用于约定宽。
* xicon兼容新版本sdk4.23
* xbutton兼容了pc状态指示。
* x-modal增加了属性zIndex，用于调整层级。调整为默认1000
* 修改了x-skeleton启动动画的延迟量，不然有可能造成低版本的安卓动画无法启动。
* x-switch-slider布局菜单样式修改了，修复部分的安卓机型菜单宽度无法设置。
* x-form修复指针空异常。由于我是采用内部字段diff比较，以提高比对性能，但有时表单字段可能存在变化，可能之前有，后面没有，造成了指针为空。
* x-picker,优化当用户提供的id值为非法值时，进行强制纠正为默认第一个值。
* x-uploadfile因为官方的css不支持word-break，导致一些文件名全是英文和数字的无法断行。目前app采取1行省略号的方法。
* x-upload-media增加了sourceType选项。
* x-share升级支持IOS支持分享文件，视频，音频，图片到微信/QQ
* [新增组件]x-pagination翻页器，用于列表翻页展示使用。
* 修复image-group预览属性无法关闭
* x-upload-midia添加了压缩属性，同时修复了自动上传关闭后，无法通过ref手动上传。
* x-modal增加contentPadding，控制内容区域的间隙。
* x-qrcoder组件更换了qr库，请更新时记得更新hybrid目录。如果你是使用的x-qrcoder-u无需要变动.
* x-image,修改了暗黑时的相关样式为统一层级关系
* x-form-item增加了labelAlign对齐属性
* x-upload-media添加了上传按钮的位置可以放置在前或者后，同时修改了排序的视觉动画，改成了拖动修改顺序。
## 1.0.9（2024-07-15）
* 请务必使用hbx 4.23 正式或者 alpha 版本。 不向下也不向上兼容。更新请重新复制 hybrid 目录 覆盖老的目录。
* [重要] **自带一些工具类库之前的xDate,xRequest,xAnimation**等等，请不要再使用原有的导入方式，请统一使用
* import {xDate } from "@/uni_modules/tmx-ui/index.uts"这种方式导入见demo。原有方式，在本版本中还未废除。预计在1.1.0中不再支持。因此你尽快改正。
* 因为我要支持加密模式。
* [重要优化] x-indexbar重构了,为了应对上千上万数据索引流畅废除了子组件indexbar-item,只保留了x-indexbar,并且使用动态插槽渲染数据,请根据demo用法重新调整你的页面.
* [新增]在工具类库能增加了tool,可以通过上述方式导出.比如防抖函数：tool.debounce，请见文档页面查看具体使用方法。
* [新增] 配置丰富限制少，x-loading-s用于在任意位置调用覆盖全屏的加载组件，完全可用来替代官方的uni.showLoading尽展现你的品牌。
* [新增] 配置丰富限制少，x-modal-s用于在任意位置调用覆盖全屏的加载组件，完全可用来替代官方的uni.showmodal尽展现你的品牌。
* [新增] 配置丰富限制少，x-toast-s用于在任意位置调用覆盖全屏的加载组件，完全可用来替代官方的uni.showToast尽展现你的品牌。
* [修复优化] x-upload-meidia组件预览索引/url有问题，由于sdk bug导致我的也有问题，改成了索引
* [优化] x-modal和darwer一样增加了延迟属性，解决ios渲染问题（sdk 底层导致，需要相关设置绕开）
* [修复] x-tabbar修复点击不跳转。
* [修复] 瀑布流进行了重构和性能优化，请务必对照demo重新使用，使用了setup方式，用户给vip用户一个setup组件使用和编写的一个示例参考。并增加了gutter间距属性。**请更新时对照demo重新优化和布局**
* [修复] switch-slider,float-button点击事件的问题。
* [修复] x-oveflay，同之前的弹层一样的问题在ios端由于底层渲染问题，导致可能打开后无法关闭。
* [优化] cell增加了labelSize属性.
* [优化增强] drawer,增加了属性：widthCoverCenter，用于适配pc，仅上下生效，让内容居中显示，不会铺满屏幕。
* [优化增强] picker系列组件,适配pc，不会撑满屏幕居中显示。
* [优化增强] x-keyboard系列组件,适配pc，不会撑满屏幕居中显示。
* [优化] slider优化了视觉
* [hybrid 修复] 修复若干插件的依赖web-view的hybrid中的jsbug请大家一定要更新这个文件夹，更新时，删除原有。直接复制本目录。
* [增强] xMarkdown富文本已经增强，高可以根据内容自适应。
* [增强] xMarkdown富文本已经可以进行内容的基础编辑，获取和设置请看demo如何使用。
* [优化] x-popover优化了弹出动画的流畅性。
* [适配] x-sign-board适配了4.22 sdk。不向下兼容
* [优化] x-button,x-tag 在skin=text模式时，修复在暗黑下的视觉效果。
* [优化] 修改x-radio-button默认字号大小。
* [适配] x-circle-progress 适配4.22 sdk的改变。不向下兼容
* [优化] x-keyboard-number，优化视觉效果，扩大确认按钮大小。新增属性：max，用于控制输入最大值，如果超过此值将无法输入。
* [新增] x-skeleton 骨架屏加载组件。自由度非常高。
* [优化] 调整了x-grid默认的高，提高视觉效果。
* [优化] 调整picker-data,picker-time个位数被充为两位数。首位填充0
* [优化视觉] x.drawer,x.picker系列的视觉样式。
* [优化视觉] x-input,修改密码时的图标间距。
* [新增] x-code-input,验证码输入框
* [适配] x-steps适配了sdk 4.21，因此不向下兼容必须4.21+
* [优化插件] x-file-s文件上传组件新增maxFilseSize，以此来过滤超过此大小的文件。且不被允许上传。
* [新增] x-scrollx组件，请于滚动指示.
* [优化] x-upload-media,新增autoStart和ref方法uplaod用于手动控制上传。
* [优化] x-navbar增加属性linear-gradient，用于定义渐变背景，使用方法见demo或者文档
* [优化] x-tabbar增加属性linear-gradient，用于定义渐变背景，使用方法见demo或者文档
* [优化] x-picker-selected增加属性lazyContent，以便在内容超多时，视觉效果更为流畅。
* [优化] x-input统一设置全局的光标颜色为主题色
* [优化] x-picker-date增加属性lazyContent，以便在内容超多时，视觉效果更为流畅。
* [优化] x-picker增加属性lazyContent，以便在内容超多时，视觉效果更为流畅。
* [优化] xradio,xcheckbox增加属性iconSize允许设置中间小图标大小。
* [优化] x-darwer，添加了属性swiperLenClose，默认为0，如果大于此0时，会触发手势，下下或者左右滑动下手势距离大于此值时会关闭弹层，建议要打开时设置为80-100比较合理
* [优化] x-picker，针对model-str，如果空值，将内部首次递归渲染回显文本。如果你后台返回，就不会计算。因此如果对性能有要求的请务必让后台在首次显示时先回显文本，这样会提高渲染性能。
* [修复] x-table,表宽计算有误。
* [优化] x-dropdown新增属性offsetTop，用于自己对菜单顶部的偏移和让位。
* [优化] x-picker放开一个原本的单位属性：cellUnits它和picker-view是同等用法的。用于给选项在顶部显示单位。
* [新增] x-keyboard-idcard 身份证键盘，会自动校验输入值，解析第二代身份证号是准确并在事件中携带校验值。
* [新增] x-keyboard-car 车牌键盘
* [优化] x-dropdown-item 允许动态更新标题字号颜色等。
* [优化] 调整了一些组件的默认圆角值
* [优化] 调整了button的默认round为空时，根据 默认的size来调整
* [优化] xEchart图表 在原有的ref函数cahrtActions增加第三个参数fun用于在调用click时的回调，请结合demo示例使用。
* [重要的修复] x-picker-selected我改变了vmodel类型改成了any[]，因此如果你是vmodel来绑定数据不会有任何变化。但如果你是confirm来读取的会有问题你读取到地ids是any[]之前是number[]因此会报类型不匹配你应该读取到ids后再as number[]或者string[]来兼容
* [优化] x-form提供了一个ref函数valid，具体见文档，用于校验指定字段，空数组校验所有。
* [优化] x-search,新增了border及darkBorderColor属性，请根据文档使用。
* [优化] x-watermark没有最长边计算错误，导致宽屏时可能覆盖不全。
* [优化] x-cell增加了属性bottomBorderInsert，用于让底边线是贯穿还是向右靠齐。同时添加了avatarRound左侧图标/头像圆角，默认6
* [修复] x-collapse可能的逻辑错误
* [swiper] 重构，减少问题，并且适应了 pc 端，现在可以用鼠标滑动了。 并且新加了多种播放模式。
* [兼容] x-popover兼容4.23 sdk 的 bug
* [优化] x-picker-date,兼容用户可能的错误值，比如开始和结束设置后，在 modevalue 时又提供个不在范围内的值，造成混乱。
* 全新的文档更新啦 支持搜索 组件，标注源码位置方便查找。
* 一些其它的已知问题。
* 修改了 demo 示例
## 1.0.8（2024-06-07）
* **本文件更新可能较多，请根据git拉取的代码比对修改，必须 sdk 4.19版本使用**
* **新的版本要求你重新复制hybrid和theme.json文件到根目录**
* [新增组件] x-markdown,允许markdown标记渲染和html文本渲染，编辑功能未开放。
* [新增api] 新增原生组件x-vibrate-s适配了ios,安卓，震动手机功能
* [新增api] 新增原生组件x-wechat-s适配了安卓，ios未适配，通过本应用拉起微信客户端指定的小程序，插件见源码。
* [新增api] 新增原生组件x-share-s适配了ios,安卓，用于任意文件文本等分享到其它应用。具体见：[分享功能](https://ext.dcloud.net.cn/plugin?id=18494)
* [增强] 原生组件x-file-s已经兼容适配了ios
* [新增组件] x-view-tofull动态全屏，效果见demo，比较酷炫
* [修复]xform校验兼容可能的类型错误，input请按照demo只能是string类型，但type可以写官方的number，让用户只输入数字。但类型需要定义为string
* [修复]所有弹出来picker由于官方本身的sdk机制推出的pageJson，我适配时出了bug，导致弹出后，再关闭导致页面可以下拉刷新了。
* [优化增强] x-form，由于官方不断完善，本组件demo已经更新对于数据绑定的vmodel数据模型已由原来的utsobjson改为了object as any，因此你前台的数据模型可以使用自己定义的结构或者utsjsonobj
* 已大幅度优化了数据模型的绑定，之前的限制已经解除，更接近了原生太的绑定了。
* [优化] xnavbar，属性activeBgColor，默认设置为空了，默认取全局的color，如果自己填写了就以填写的为准。
* [优化] input左图标颜色，iconColor，默认设置为空了，默认取全局的color，如果自己填写了就以填写的为准。
* [修复] xformitem demo演示页面属性showBottomBorder拼写成了showBottomBoder写错了，同时属性判断有问题。
* [优化] demo首页不再使用官方的listview，这个官方组件表现的不尽人意，兼容性有待考验。
* [优化] darg，在app端增加了启动排序时触发手机震动,并改成了长按触发。
* [优化] switchSlider，让滑动页面和左滑不冲突，体验更良好。
* [优化] xRequest请求失败时把statusCode赋值为errCode。
* [修复] cell的hover效果消失了。
* [优化] xnavbar,添加了showNavBack属性，是否隐藏返回按钮。
* [优化] x-float-drawer增加了scroll容器的事件协商，不会干扰滚动内容。同时增加了头点击时展开或者关闭的功能。
* [优化] x-tabs子项目超多进，会让原生的安卓无法执行节点查询系sdk bug，我已经作 了兼容来绕开该问题。
* [优化] x-input，添加了showChartCount属性用来控制是否要显示字符统计。
* [优化] x-image，改动较大，主要是做了响应式处理，因为image原生是不支持%,auto，我是模拟支持了，因此如果出现异常请把你的布局代码提交到conding.net上
* [修复] x-calendar,修复了在web平台点击本日时，不会选中当日的bug.日历格式把个位数补充为0
* [优化] xReauest增加了一个参数successStatusCode,用于定义服务器自己成功返回的code，默认是200
* [优化] ios,web，前台和后台切换时，可能导致轮播的定时器异常混乱，导致轮播图片异常。已作兼容。
* [优化] xuploadMedia添加了complete，change事件。
* [优化] x-cell添加了bottomBorderColor，用于自定边线颜色。
* [优化] x-xCascader标题过多时增加了左右滚动。
* [优化] ios端暗黑弹层调整，因兼容sdk的 pullrefresh导致暗黑切换异常。
* [优化] radio,checkbox适当增加了4px大小 。
* [优化] switch调整了其宽和高，默认size高与input对齐，排版时更美观。
* [增强] x-image-vip增加 属性：isScanQr，开启后，可长按图片识别二维码，类似微信的长按功能。（普通授权用户无法使用）
* [修复] x-notice，如果切换到下一页时，由于sdk在web回收机制导致返回时功能失效。现已修复。
* [优化+增强] x-navbar添加了事件fiexdChange，与顶部与差比0-1，0表示在顶，1表示超过了自身高。使用方法见demo示例，展现了元素之间差值计算
* [优化] x-formitem,x-form增加showLabel属性，用于隐藏标题。
* [增强] x-files-s,x-upload-file对应的原生文件和组件文件选择器，已经适配了ios系统11+（含），现在三端已经对齐可在不同平台选择任意文件上传了。
* [增强] x-ehcart 图表新增ref函数cahrtActions，用于执行echart对象任意函数方法，具体见文档。同时适配响应式尺寸。现支持横屏和和竖屏了。
* [优化] x-sign-board，现在适配响应式尺寸，支持横屏和竖屏切换。
* [优化] x-upload-media，现在适配响应式尺寸，支持横屏和竖屏切换。
* [优化] x-table，支持具名指定单元格插槽及行插槽，用于复杂的布局（使用插槽时，不可排序），现在你可以在单格内放置按钮，输入框等一切布局元素了。
* [优化] x-image-group,增加了round圆角属性。
* [优化] x-popover，增加了打开动画.
* 其它未记录到的变化，请根据git对比差异。
*
## 1.0.7（2024-05-07）
* **升级说明：改动较大，请按照文档或者demo重新配置相关插件路径**
* 1.已适配IOS端
* 2.已全局适配字号设置，可全局动态修改字号大小（方便适配老人版本）
* 3.已适配暗黑效果（IOs目前存在点问题待官方修复）
* 4.已自动适配横屏，竖屏，宽屏，电脑端，可下载demo自动旋转屏幕查看效果及打开电脑web查看效果
* 5.部分原生插件适配了ios端
* **至此 安卓，WEB|H5，IOS多端已完成适配**
* 接下下我将适配剩余的插件到ios,web端，及维护，细节优化,及扩展更多插件及功能。
-------------
* 【重要】[xOcrS离线识别](https://ext.dcloud.net.cn/plugin?id=17557)文本识别插件已经支持ios12.0(含)+
* 【重要】[xQrU码生成](https://ext.dcloud.net.cn/plugin?id=16470)已经支持IOS9.0(含)+,高效，批量无延迟生成码
* [新增] 新增文件选择器[在此围观](https://ext.dcloud.net.cn/plugin?id=17624),tmui4.0种子用户已经附赠在源码中（无需购买），请更新版本后直接按照文档引用即可。如果需要查看效果请下载本应用demo，导航到原生插件栏目体验。
* [新增] x-vibrate-s震动api，让手机震动（安卓，ios），web不支持。
* [扫码插件] 原先的scning-u改成了嵌入式扫码可自行像页面布局一样装饰你的相机扫码功能。
* [新增扫码插件] 原先的scning-s为api试扫码功能与上面的嵌入式不一样。这个是通过api方式调用。可以在任意Uts位置调用识别。
* [新增] x-waterfall瀑布流组件。
* [增强] x-table增加一个属性multiRowFloat用于是否开启多行固定表头的功能（滚动指定行时自动吸顶在顶部）。
* [增强] x-rate,增加属性half，打半星的功能。允许打半分。
* [优化] x-picker-selected，新增modelStr回显已选文本，请使用v-model:model-str来同步显示。并修改了样式。
* [优化] x-slider-menu添加属性showScrollbar是否显示滚动条
* [优化] x-form表单的默认校验兼容性（兼容sdk的bug）
* [修复] x-request请求库中的某个bug问题。
* [修复] xDate时间不能早于1950年的问题。及小时的问题
* [优化] xtabs支持屏幕响应变化。
* [优化] x-slider-tree调整，返回时应该不触发change的。
* [修改] x-float-button关闭了首次的默认动画效果。
* [修复] x-image，在计算占位时可能的设置宽高有问题导致意外的为0宽（缓存加载时安卓端易出现。）
* [修复] xtabs在安卓及ios端时（web正常），首次进入渲染修改底部线条位置和宽时不会有效，经过测试界面上是有相关宽和属性但就是不显示可能与sdk或者其它因素导致。目前通过vif切换刷新解决。
* [优化] x-tabbar样式优化和修改。
* [优化] x-picker-view安卓端适配4.13sdk
* [优化] x-search增加了取消显示和隐藏的缩放动画
* [优化] x-popover在h5端可能的位置偏移。
* [优化] x-more增加了click事件，用于点击打开/关闭是触发，方便写事件是否允许打开查看详情内，比如付费解锁内容/登录查阅内容场景。
* 其它的修复优化
## 1.0.6（2024-04-10）
* 【重要】更新后请使用4.06+ sdk正式版本，不可使用4.11 dev或者4.11 aplpha版本（4.11 bug 太多官方在修复中）
* 【重要】新增xOcrS Api插件用于图片文字识别，支持网络，本地，相册，拍照识别功能。[在此围观](https://ext.dcloud.net.cn/plugin?id=17557)
* tmui4.0种子用户已经附赠在源码中（无需购买），请更新版本后直接按照文档引用即可。如果需要查看效果请下载本应用demo，导航到原生插件栏目体验。
* [优化] x-floatDrawer 在safri web浏览器上可能的尺寸不正确。
* [优化] x-picker,x-picker-view添加了model-str用于选中项回显显示文本。同时修复了历史遗留问题（4.02之前sdk的问题导致）
* [优化] 调整了xbadge的样式。
* [优化] xrequest把参数showLoadToast，showSuccessToast，showErrorToast提取了全局配置中，可通过静态方法setShowToast统一设置，使得全局请求统一行为，局部可以覆盖全局。
* [优化] pickder,picker-data,time等添加confirm事件，超长文本断行问题。
* [优化] inputTexarea，把默认的confirmType改成空值。否则会不能断行。
* [修复] formItem属性showBottomBorder并写错误已改正，已使用的请改下，看文档复制下。
* [新增] 新增xTable表格组件。
* [优化] 部分组件类型统一放置在interface中,方便用户自己导入使用类型。
* [优化] x-sticky之前兼容老的sdk后，忘记在新sdk中改回
* [优化] x-cell,rightLabel做了限制修改,同时添加了title插槽
* [新增] x-devtool开发组件，主要是记录请求的一些记录。开发者可以将此组件预埋在某个页面，通过后台请求是否打开 和显示开发模式。这样在app发布后，通过远程调试也可打开请求记录等一些实质的数据。
* [优化修复] x-float-drawer，onlyHeader为true后在pc端可能造成拖动不了。
* [优化] x-float-button兼容pc电脑端。同时改进了设计窗口尺寸方案。
* [优化] x-tabs，增加了titlePadding两边文字间隙属性。同时修复之前兼容旧sdk问题，本次更新不兼容旧sdk请4.06+
* [新增] x-row,x-col组件，用于常规布局使用。
* [新增] 城市选择器 xPickerCity
* [优化] ximage添加了round圆角属性。
* [优化] xinput增加了inputPadding属性用于控制输入框的padding值。
* 其它代码的优化和修复。
## 1.0.5（2024-04-03）
* [优化] x-drag list类型改成了UTSJSONObject[],这样可以不限制你的list类型。
* [优化和修复] action-menu,新增cellClickClose属性可以控制选项是否能关闭弹层。同时修复设置了maxHeight时，头尾不见了。
* [新增] 增加了xRequest 请求库
* [优化]switch-slider，优化了误触时的左右滑动距离。
* [优化] 时间类组件，默认格式化后给加上了10进位占位0，并且默认如果为非正常时间值（比如空，或者 不合规范的值）将不再同步更新掉，而是不操作和纠正。
* [优化] x-tag优先height，而不是size,因此默认size为空
* [优化] modal,drawer加了滑动阻止，之前忘记加了。
* [优化] x-icon增加属性duration，动画时间的控制。
* [优化] xInput，补充了一些官方的事件和属性，增加了texarea文本域
* [优化] xSwiper，默认把动画时长 改小了。如果默认为1数量，禁用自动轮播。
* [增强] xSwiper，添加了space属性，用于控制两边露出的距离，形成卡片试轮播。
* [新增] xcard组件
* 其它代码的优化和修复
## 1.0.4（2024-03-28）
* **重要：升级后请务必uniapp x sdk4.05+版本**这个版本修复了我提到了查询延迟导致部分组件兼容影响视觉感观。
* **组件适配了web端，也就是说可以用来写电脑端，也可以写移动端。**
* [新增] 新增组件x-drag自由拖拽排序组件，自由度高布局不限制。
* [新增] 搜索列表组件picker-selected，可支持本地数据搜索或异步加载列表搜索
* [xAnimate] 新增属性isDescPlay,所有按顺序加入的动画元素，将顺序执行动画，而不是一起执行,
* 新增enterFrame渲染回调函数，可用于实时动画操作或者绘图操作。类似canvas渲染，可作一些动画交互，视觉体验的增强
 [效果预览](https://xui.tmui.design/h5/pages/index/xanimate)
* [rate] 增加了交互动画
* [overlay] 在禁用遮罩关闭时，增加了交互动画弹跳效果以视觉方式让用户知道无法关闭。并支持了嵌套，需要sdk4.02+
* [x-upload-media] 优化长按拖动图片排序时在web间易误触发预览图片。
* [switch-slider] 暂时关闭默认插槽数据传递，原因是4.04bug导致待官方修复放开数据。
* [indexbar] 优化了下时间段和样式，把节流时间调低。
* [modal] 在禁用遮罩关闭时，增加了交互动画弹跳效果,并支持了嵌套，需要sdk4.02+
* [x-action-menu] 样式调整，行加高，遮罩颜色 加深
* [x-sign-board] 兼容了pc浏览器，方便在pc端使用。
* [x-keyboard-number] 确认键改成了中文字符，不使用图标，方便国人。
* [增强] 所有弹出层等一些界面支持响应式屏幕,以便支持pc及宽屏ipad
* [优化] 优化indexbar性能
* [优化] 优化slider-double支持电脑，及修复意外的布局错误。
* [优化] 优化button，字号的判断错误，默认设置为空，取size的字号。
* [xDate] 优化日期处理的时间，提升性能，性能比对是能提高30ms左右
* [优化] x-calendar-view日历组件，提升了响应时间，比之前快50%,之前大概触摸到结束响应时间（如果遇到自定样式多时）大概在250-350ms之间，造成部分手机不跟手。优化后大概是60-110ms之间。
* 在此也提醒各位：数据操作中尽量不要使用findex,indexOf，includes等判断和筛选。应该使用基础的for然后break;
* 4.05的sdk发布，对应的作了一些兼容。
## 1.0.3（2024-03-18）
* [非常重要] **本次升级需要要4.02+uniappx sdk支持，并适配了web版本。改动较多且较大，日志不反映实际更改，只列出重要的更改**
* [更新事项] 更新时，请注意同步更新hybrid目录文件。
* [优化] x-action-menu新增space属性，可以控制是否让两边和底部留下空白。默认为true
* [优化] x-sign-board删除了组件的引用，使用自己的事件。
* [新增x动画库] xAnimate ,核心功能：支持官方所支持的css属性动画，支持联合属性动画和单独控制，支持事件回调（运行时，结束时，开始时），它并非是css animation动画
* 而是我写的js动画，并且在web,安卓端采用原生的刷新机制（并非settimeout实现）因此保证了超高的性能优势。并且让你做一些交互动画时异常简单。效果请下载demo或者线上看web。进入首页
* 拉到底部进入xAnimate页面查看你的手机效果对齐流畅度。具体用法请参考demo页面使用及文档参数
* [增强] x-finger新增pinch捏合缩放，roate两指旋转事件，通过事件可获取当前的缩放和旋转信息。
* [新增] x-calendar,日历面板,app端使用canvas绘制，web端使用节点绘制
* [优化增强]picker系列组件，统一采用虚拟列表，因此可以支持大量数据，性能更强。
* [优化增强]极联无限级选择器，已优化使用虚拟列表，可支持无限大数据量,性能更强。
* [优化增强] sliderTree，列表全部改为虚拟列表，可支撑大量数据。
* [优化增强] uploadMedia上传组件，已支持拖拽排序，激活条件是长按图片且未在上传时激活排序。主要是方便产品的封面图顺序排列更为方便。
* [优化增强] drawer，增强disabledScroll属性，可以用来禁用（内容超长时）滚动，以阻uniappx sdk的自身bug:1,套input不会上推键盘,2,view move事件打断。
* [优化] pickview下pickviewitem里面有两个版本，目前使用的是官方的pick-view封装，如果觉得不好用，可以使用我自己编写的pick-view，只要内容互换就行，自编的可控度高。
## 1.0.2（2024-01-25）
【新增】：x-mlkit-scannig-u原生扫码组件，支持多码识别。采用谷歌机器学习库识别速度非常快。具体见demo自己体验效果，也可以根据需求自行修改定制（当前交互类微信）[插件地址](https://ext.dcloud.net.cn/plugin?id=16468)

【新增】：x-qr-u原生生成码组件，与原有的x-qrcoder不同，qrcoder是webview实现的。本qr-u是原生库，效率流畅无比，支持13种码制格式。
修改了部分样式[插件地址](https://ext.dcloud.net.cn/plugin?name=x-qr-u)

【优化】：针对真机，修改改部分样式及字体大小等。

【声明】：当前sdk3.99 uniappx还不适合应用于生产，得让官方修复bug，下个版本应该有所好转。

**特别注意：带-u的是原生组件插件带-s是原生插件api，类目中x-core为原生UTs实现的基类**

虽然购买能获得源码，但原生插件类组件，要求门槛较高，购买可以边学习边生产使用，可自行研究修改。view组件是正常的应用布局使用。

## 1.0.1（2024-01-16）
试用能下载源码，请直接微信找我要源码
## 1.0.0（2024-01-16）
tmui4.0 xui 1.0.0初发版本号。约82个组件。

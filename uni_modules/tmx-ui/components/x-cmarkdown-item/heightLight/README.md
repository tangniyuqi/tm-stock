# 代码高亮语言配置

本目录包含了世界主流编程语言的语法高亮配置文件。

## 支持的语言

本目录支持以下20种编程语言：

1. **Python** (`python.uts`) - 动态类型编程语言
2. **Java** (`java.uts`) - 面向对象编程语言
3. **C#** (`csharp.uts`) - .NET框架编程语言
4. **C++** (`cpp.uts`) - 系统级编程语言
5. **C** (`c.uts`) - 系统编程语言
6. **TypeScript** (`typescript.uts`) - JavaScript超集
7. **PHP** (`php.uts`) - Web开发语言
8. **Swift** (`swift.uts`) - iOS/macOS开发语言
9. **Kotlin** (`kotlin.uts`) - Android开发语言
10. **Go** (`go.uts`) - Google开发的并发编程语言
11. **Ruby** (`ruby.uts`) - 动态面向对象语言
12. **Rust** (`rust.uts`) - 系统级内存安全语言
13. **Objective-C** (`objc.uts`) - iOS/macOS开发语言
14. **Dart** (`dart.uts`) - Flutter开发语言
15. **SQL** (`sql.uts`) - 数据库查询语言
16. **R** (`r.uts`) - 统计计算语言
17. **Perl** (`perl.uts`) - 文本处理语言
18. **Scala** (`scala.uts`) - JVM函数式编程语言
19. **Bash/Shell** (`bash.uts`) - Unix Shell脚本
20. **HTML** (`html.uts`) - 网页标记语言

## 使用方法

### 方法一：使用统一接口

```typescript
import { getLanguageConfig } from "./heightLight/index.uts"
import { highlightCode } from "./heightlight.uts"

// 获取特定语言的配置
const config = getLanguageConfig('python')

// 使用配置进行代码高亮
const code = "def hello():\n    print('Hello, World!')"
const styles = { /* 样式配置 */ }
const result = highlightCode(code, config, styles)
```

### 方法二：直接导入语言配置

```typescript
import { getPythonConfig } from "./heightLight/python.uts"
import { highlightCode } from "./heightlight.uts"

const config = getPythonConfig()
const result = highlightCode(code, config, styles)
```

### 方法三：默认配置（JavaScript）

如果语言不在支持列表中，或者未指定语言，系统会使用默认的 JavaScript 配置：

```typescript
import { highlightCode } from "./heightlight.uts"

// 传入 null 作为配置，将使用默认的 JavaScript 配置
const result = highlightCode(code, null, styles)
```

## 语言别名

`getLanguageConfig()` 函数支持以下语言别名（不区分大小写）：

- **Python**: `python`, `py`
- **Java**: `java`
- **C#**: `csharp`, `c#`, `cs`
- **C++**: `cpp`, `c++`, `cplusplus`
- **C**: `c`
- **TypeScript**: `typescript`, `ts`
- **PHP**: `php`
- **Swift**: `swift`
- **Kotlin**: `kotlin`, `kt`
- **Go**: `go`, `golang`
- **Ruby**: `ruby`, `rb`
- **Rust**: `rust`, `rs`
- **Objective-C**: `objectivec`, `objective-c`, `objc`, `obj-c`
- **Dart**: `dart`
- **SQL**: `sql`, `mysql`, `postgresql`, `postgres`, `sqlite`
- **R**: `r`
- **Perl**: `perl`, `pl`
- **Scala**: `scala`
- **Bash**: `bash`, `sh`, `shell`, `zsh`
- **HTML**: `html`, `htm`

## 配置结构

每个语言配置文件都包含以下三个部分：

1. **keywords**: 语言关键字数组
2. **builtins**: 内置对象、类型、函数数组
3. **operators**: 操作符数组

示例：

```typescript
export class HighlightConfig {
    keywords: string[] = ['if', 'else', 'for', ...]
    builtins: string[] = ['Array', 'Object', ...]
    operators: string[] = ['+', '-', '*', ...]
}
```

## 添加新语言

如果需要添加新的语言支持：

1. 在本目录下创建新的 `.uts` 文件（例如：`newlang.uts`）
2. 定义并导出配置函数，返回 `HighlightConfig` 对象
3. 在 `index.uts` 中导入并添加到 `getLanguageConfig()` 的 switch 语句中
4. 更新 `getSupportedLanguages()` 函数的返回数组

## 注意事项

- 所有语言配置文件都从 `../heightlight.uts` 导入 `HighlightConfig` 类
- 如果 `getLanguageConfig()` 返回 `null`，请使用默认配置（JavaScript）
- 配置是只读的，每次调用都会返回新的配置对象


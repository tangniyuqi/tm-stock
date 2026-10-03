// Package guard 是 AI 分析模块的确定性校验器链。
//
// 设计依据：docs/specs/ai-analysis/design.md 第 3 节。核心思想不是"把模型违规的概率压低"，
// 而是让概率性碰不到展示层：模型的输出必须先过这里的确定性检查，用户看到的文字只能是
// 代码按模板渲染的句子，或绑定证据编号的逐字引文。
//
// 本包提供的能力（均为纯函数，无 IO、无全局可变状态）：
//
//   - Normalize：来源与模型输出进入系统前的规范化（NFKC、去零宽字符、空白折叠）；
//   - SplitSentences / Snapshot：快照按句切分编号，引文由代码按编号取原文（模型不返回自由文本引文）；
//   - CheckText：红线文本检测（词表词 + 句式规则，抗插字、全角、繁体、换行拆词等规避）；
//   - ScreenQuote：引文展示前筛查，命中则截取不含命中的最小事实子句，截不出就转人工；
//   - CoOccur：实体与题材锚词须同句共现（专治"推断出来的关联"）；
//   - NumbersGrounded：展示的数字必须逐字出现在引文或结构化字段里；
//   - ValidateExtraction：对模型返回的抽取结果做封闭结构校验并逐条裁决。
//
// 词表的单一真源是 scripts/compliance-forbidden-words.txt；本包的 words.txt 是构建时的副本，
// 由 scripts/sync-guard-words.sh 同步，并由 TestWordsMatchSource 与 CI 守卫保证一致。
//
// 编码字符引用（HTML 数字实体、\u 转义）不解码，而是"出现即拦"：正常文本里不会有这些写法，
// 它们的唯一用途是把词表词藏起来；规则写在 patterns.txt，用 @raw 标记在规范化视图上匹配。
//
// 重要边界（诚实说明）：确定性规则可审计、可复现，但拦不住谐音、拆字外的同音替换、拼音与 emoji 替代、
// 语义改写，以及 Base64、百分号编码等其余编码型变形。这些靠设计里的第二道防线（语义分类器 + 人工抽检，
// design.md 3.4.1）兜底，红队集里以 known_gap 标出，不在本包的承诺范围内。
package guard

// 节选依据展示的可重复测试；不替代 HBuilderX 的 uni-app x 整体编译与浏览器验收。
// 用法：node scripts/dev/verify-evidence-ui.cjs <临时依赖目录>
// 依赖：vue、@vue/compiler-sfc、typescript（与项目分离安装，不修改业务依赖）。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');
const root = path.resolve(__dirname, '../..');
const runtime = process.argv[2] ? path.resolve(process.argv[2]) : path.join(root, 'backend/web');
const dependency = createRequire(path.join(runtime, 'package.json'));
const ts = dependency('typescript');
const compiler = dependency('@vue/compiler-sfc');
const vue = dependency('vue');
const { renderToString } = dependency('@vue/server-renderer');
const read = file => fs.readFileSync(path.join(root, file), 'utf8');

function execute(source, context = {}) {
  const output = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
    reportDiagnostics: true,
  });
  const errors = output.diagnostics.filter(item => item.category === ts.DiagnosticCategory.Error);
  assert.deepEqual(errors, [], '测试源代码无法转换');
  const sandbox = { exports: {}, require: dependency, ...context };
  vm.runInNewContext(output.outputText, sandbox, { timeout: 1000 });
  return sandbox.exports;
}

function parseSFC(file) {
  const result = compiler.parse(read(file), { filename: file });
  assert.deepEqual(result.errors, [], `${file} 解析失败`);
  compiler.compileScript(result.descriptor, { id: 'evidence-ui-qa' });
  return result.descriptor;
}

async function main() {
  const external = execute(read('pages/libs/external-link.uts'));
  const theme = execute(read('pages/libs/theme.uts'));
  const detail = parseSFC('pages/segment/detail.uvue');
  // 从实际页面 AST 抽取验证器，不能复制一个测试版验证器骗过测试。
  const tree = ts.createSourceFile('detail.ts', detail.scriptSetup.content, ts.ScriptTarget.Latest, true);
  const names = new Set(['isSafeTsCode', 'isPublishableEvidence']);
  const validators = tree.statements.filter(statement => ts.isVariableStatement(statement)
    && statement.declarationList.declarations.some(declaration => names.has(declaration.name.getText(tree))));
  assert.equal(validators.length, 2);
  const { isPublishableEvidence } = execute(validators.map(item => item.getText(tree)).join('\n')
    + '\nexports.isPublishableEvidence = isPublishableEvidence;', { isSafeExternalUrl: external.isSafeExternalUrl });
  const evidence = {
    tsCode: '688001.SH', stockCode: '688001', stockName: '测试公司', chainNodeName: '测试环节',
    sourceType: '年报', sourceExcerpt: '测试：公司披露主营算力服务。', excerptTrimmed: true,
    sourceUrl: 'https://example.test/report', collectedAt: 1791580800000,
  };
  for (const flag of [true, false]) assert.equal(isPublishableEvidence({ ...evidence, excerptTrimmed: flag }), true);
  for (const flag of [undefined, null, 0, 1, 'false', 'true']) {
    assert.equal(isPublishableEvidence({ ...evidence, excerptTrimmed: flag }), false, '缺失/错误类型标记必须拒绝展示');
  }
  for (const bad of [{ sourceExcerpt: '' }, { sourceUrl: 'javascript:alert(1)' }, { collectedAt: 0 }]) {
    assert.equal(isPublishableEvidence({ ...evidence, ...bad }), false);
  }
  const sheet = parseSFC('pages/segment/components/ThemeEvidenceSheet.uvue');
  const compiled = compiler.compileTemplate({
    source: sheet.template.content, filename: 'ThemeEvidenceSheet.uvue', id: 'evidence-ui-qa',
    compilerOptions: { isCustomElement: tag => ['view', 'text', 'scroll-view'].includes(tag) },
  });
  assert.deepEqual(compiled.errors, [], '依据浮层模板编译失败');
  const { render } = execute(compiled.code);
  const renderEvidence = item => renderToString(vue.createSSRApp({
    render,
    setup: () => ({ evidence: item, visible: true, close: () => {}, openSource: () => {},
      isSafeExternalUrl: external.isSafeExternalUrl, formatMillis: theme.formatMillis }),
  }));
  const trimmed = await renderEvidence(evidence);
  assert.ok(trimmed.includes('原文摘录（节选）'));
  assert.ok(trimmed.includes('已省略部分原文，请结合原文链接核对完整上下文。'));
  assert.ok(trimmed.includes(evidence.sourceExcerpt));
  const whole = await renderEvidence({ ...evidence, excerptTrimmed: false });
  assert.ok(!whole.includes('原文摘录（节选）') && !whole.includes('已省略部分原文'));
  const injection = await renderEvidence({ ...evidence, sourceExcerpt: '<script>测试</script>' });
  assert.ok(injection.includes('&lt;script&gt;测试&lt;/script&gt;') && !injection.includes('<script>测试</script>'));
  for (const file of ['backend/web/src/view/quant/themeStock/themeStock.vue', 'backend/web/src/view/quant/themeStock/themeStockForm.vue']) {
    const descriptor = parseSFC(file);
    const output = compiler.compileTemplate({ source: descriptor.template.content, filename: file, id: 'gva-evidence-ui-qa' });
    assert.deepEqual(output.errors, [], `${file} 模板编译失败`);
  }
  console.log('通过：实际页面验证器正反例、节选/非节选 SSR 文案、原文转义、两处 GVA 表单 SFC 编译。');
  console.log('未验证：HBuilderX 整体构建、真实浏览器布局与点击交互。');
}
main().catch(error => { console.error(error); process.exitCode = 1; });

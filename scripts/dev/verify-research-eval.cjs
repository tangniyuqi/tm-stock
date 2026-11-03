// 检查真实Node测试摘要，不能把零用例、跳过或取消当作评测工具已验收。
const {spawnSync} = require('node:child_process');
const path = require('node:path');
function validTAP(output) {
  const fields = ['tests','pass','fail','cancelled','skipped','todo'];
  const values = {};
  for (const field of fields) {
    const found = [...output.matchAll(new RegExp('^# '+field+' ([0-9]+)$','gm'))];
    if (found.length !== 1) return false;
    values[field] = Number(found[0][1]);
    if (!Number.isSafeInteger(values[field])) return false;
  }
  const passes = [...output.matchAll(/^ok [0-9]+ - /gm)].length;
  return values.tests >= 27 && values.pass === values.tests && passes === values.tests && ['fail','cancelled','skipped','todo'].every(k=>values[k]===0);
}
function main() {
  const child = spawnSync(process.execPath,['--test','--test-reporter=tap',path.join(__dirname,'research-eval.test.cjs'),path.join(__dirname,'research-qa-capture.test.cjs')],{encoding:'utf8',maxBuffer:2*1024*1024});
  if (child.status !== 0 || !validTAP(child.stdout || '')) {
    process.stderr.write((child.stdout || '').slice(-6000)+(child.stderr || '').slice(-2000));
    process.stderr.write('\n资料研究离线评测工具自测未完成。\n');
    process.exitCode = 1; return;
  }
  const passed = /^# pass ([0-9]+)$/m.exec(child.stdout)[1];
  process.stdout.write('通过 '+passed+' 项 / 失败 0 项（工具自测，非模型评测）\n');
}
module.exports = {validTAP};
if (require.main === module) main();

// 执行原UVue登录脚本的异步事件；不渲染模板，不替代H5/真实鉴权验收。
// 用法：node scripts/dev/verify-member-login-script.cjs <仓库外隔离依赖目录>
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const {createRequire}=require('node:module');
if(!process.argv[2])throw new Error('必须提供隔离依赖目录');
const dependency=createRequire(path.join(path.resolve(process.argv[2]),'package.json'));
const compiler=dependency('@vue/compiler-sfc'),ts=dependency('typescript'),vue=dependency('vue');
const filename=path.resolve(__dirname,'../../pages/member/login.uvue');
const parsed=compiler.parse(fs.readFileSync(filename,'utf8'),{filename});assert.deepEqual(parsed.errors,[]);
const script=compiler.compileScript(parsed.descriptor,{id:filename});
const output=ts.transpileModule(script.content,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020},reportDiagnostics:true});
assert.equal(output.diagnostics.filter(d=>d.category===ts.DiagnosticCategory.Error).length,0);
let completed=false;
process.once('beforeExit',()=>{if(!completed){console.error('原登录脚本用例未完整结束');process.exitCode=1}});
(async()=>{
  // 执行原请求封装；传输替身模拟框架先解析JSON的行为，不能把纯文本错误当成功数据。
  const requestFile=path.resolve(__dirname,'../../api/request.uts');
  const requestOutput=ts.transpileModule(fs.readFileSync(requestFile,'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020},reportDiagnostics:true});
  assert.equal(requestOutput.diagnostics.filter(d=>d.category===ts.DiagnosticCategory.Error).length,0);
  for(const mode of ['json_success','object_success','business_rejection','plain_404','html_502','malformed_200','array_200','null_200','network_rejection']) {
    const notices=[];let requested,frameworkParses=0,bodyParses=0;
    const response=mode==='object_success'?{code:0,data:{nickname:'仅测试'}}:mode==='json_success'?'{"code":0,"data":{"nickname":"仅测试"}}':mode==='business_rejection'?'{"code":7,"msg":"仅测试拒绝"}':mode==='plain_404'?'404 page not found':mode==='html_502'?'<html>Bad Gateway</html>':mode==='array_200'?'[]':mode==='null_200'?'null':'not-json';
    class Request {
      static setHostUrl() {}
      async request(options) {
        requested=options;
        if(mode==='network_rejection')throw new Error('仅测试网络失败');
        let data=response;
        if(options.dataType!=='text'&&typeof data==='string'){frameworkParses++;data=JSON.parse(data)}
        const statusCode=mode==='plain_404'?404:mode==='html_502'?502:200;
        if(statusCode!==200)throw {statusCode,data};
        return {statusCode,data};
      }
    }
    const sandbox={exports:{},JSON:{parse:JSON.parse,stringify:JSON.stringify,parseObject:value=>{bodyParses++;return JSON.parse(value)}},
      uni:{getStorageSync:()=> 'test-member-token',showModal:o=>notices.push(o.title),showToast:o=>notices.push(o.title)},
      require:name=>{if(name==='@/uni_modules/tmx-ui/index.uts')return {xRequest:Request};if(name==='@/config/baseUrl.uts')return {baseUrl:'http://127.0.0.1/client'};throw new Error('未声明请求层依赖：'+name)}};
    vm.runInNewContext(requestOutput.outputText,sandbox,{filename:requestFile,timeout:2000});
    const result=await sandbox.exports.rqWithToken('/member/settings','GET',{},true).then(value=>({ok:true,value}),()=>({ok:false}));
    const success=['json_success','object_success'].includes(mode);
    assert.equal(result.ok,success,mode);if(success)assert.equal(result.value.nickname,'仅测试');
    assert.equal(requested.dataType,'text');assert.equal(frameworkParses,0);
    assert.equal(requested.header.Authorization,'Bearer test-member-token');assert.equal(requested.useCache,false);
    if(['plain_404','html_502','network_rejection'].includes(mode))assert.equal(bodyParses,0);
    if(['malformed_200','array_200','null_200'].includes(mode))assert.deepEqual(notices,['服务器异常']);
    if(mode==='business_rejection')assert.deepEqual(notices,['仅测试拒绝']);
    console.log('通过：原请求层 '+mode);
  }
  for(const mode of ['business_rejection','network_rejection','null_response','success','profile_failure','double_click']) {
    const storage=new Map(),navigation=[],toasts=[];let calls=0,profiles=0,release;
    const wait=new Promise(resolve=>{release=resolve});
    const auth={login:async()=>{calls++;if(mode==='double_click')await wait;if(mode.endsWith('rejection')){toasts.push('请求层已提示');throw null}return mode==='null_response'?null:{token:'test-member-token'}}};
    const member={getProfile:async()=>{profiles++;if(mode==='profile_failure')throw null;return null}};
    const sandbox={exports:{},getCurrentPages:()=>[{},{}],setInterval:()=>1,clearInterval:()=>{},
      uni:{getStorageSync:k=>storage.get(k),setStorageSync:(k,v)=>storage.set(k,v),showToast:options=>toasts.push(options.title),navigateBack:()=>navigation.push('back'),switchTab:options=>navigation.push(options.url)},
      require:name=>{if(name==='vue')return {...vue,onBeforeUnmount:()=>{}};if(name==='@/api/auth.uts')return {api:auth};if(name==='@/api/member/member.uts')return {api:member};throw new Error('未声明测试依赖：'+name)}};
    vm.runInNewContext(output.outputText,sandbox,{filename,timeout:2000});
    const state=sandbox.exports.default.setup({},{expose:()=>{}});state.account.value='ai_member_one';state.password.value='仅测试口令';
    const attempt=state.doLogin();
    if(mode==='double_click'){assert.equal(state.loading.value,true);await state.doLogin();assert.equal(calls,1);release()}
    await assert.doesNotReject(()=>attempt,mode+'：页面必须消费已提示的请求拒绝');
    assert.equal(state.loading.value,false);assert.equal(calls,1);
    const success=['success','profile_failure','double_click'].includes(mode);
    assert.equal(storage.has('token'),success);assert.equal(navigation.length,success?1:0);assert.equal(profiles,success?1:0);
    if(mode.endsWith('rejection'))assert.equal(toasts.length,1,'页面catch不能重复弹错');
    console.log('通过：原登录脚本 '+mode);
  }
  for(const mode of ['auth_switch','auth_unmount','profile_switch','profile_unmount','profile_rejection_switch']) {
    const storage=new Map(),navigation=[];let release,unmount;
    const pending=new Promise(resolve=>{release=resolve});
    const auth={login:async()=>{if(mode.startsWith('auth_'))await pending;return {token:'old-login-token'}}};
    const member={getProfile:async()=>{if(mode.startsWith('profile_'))await pending;if(mode==='profile_rejection_switch')throw null;return {nickname:'旧会员资料',mobile:'test',credit:9,level:1}}};
    const sandbox={exports:{},getCurrentPages:()=>[{},{}],setInterval:()=>1,clearInterval:()=>{},
      uni:{getStorageSync:k=>storage.get(k),setStorageSync:(k,v)=>storage.set(k,v),showToast:()=>{},navigateBack:()=>navigation.push('back'),switchTab:()=>navigation.push('tab')},
      require:name=>{if(name==='vue')return {...vue,onBeforeUnmount:fn=>{unmount=fn}};if(name==='@/api/auth.uts')return {api:auth};if(name==='@/api/member/member.uts')return {api:member};throw new Error(name)}};
    vm.runInNewContext(output.outputText,sandbox,{filename,timeout:2000});
    const state=sandbox.exports.default.setup({},{expose:()=>{}});state.account.value='ai_member_one';state.password.value='仅测试口令';
    const attempt=state.doLogin();await Promise.resolve();await Promise.resolve();
    if(mode.endsWith('unmount'))unmount();else storage.set('token','new-account-token');
    release();await attempt;
    assert.equal(navigation.length,0,mode);assert.equal(storage.has('nickname'),false,mode);
    if(!mode.endsWith('unmount'))assert.equal(storage.get('token'),'new-account-token',mode);
    if(mode==='auth_unmount')assert.equal(storage.has('token'),false);
    console.log('通过：原登录迟到隔离 '+mode);
  }
  const memberFile=path.resolve(__dirname,'../../pages/member/index.uvue');
  const memberParsed=compiler.parse(fs.readFileSync(memberFile,'utf8'),{filename:memberFile});assert.deepEqual(memberParsed.errors,[]);
  const memberScript=compiler.compileScript(memberParsed.descriptor,{id:memberFile});
  const memberOutput=ts.transpileModule(memberScript.content,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020},reportDiagnostics:true});
  assert.equal(memberOutput.diagnostics.filter(d=>d.category===ts.DiagnosticCategory.Error).length,0);
  for(const mode of ['guest','settings_rejection','settings_null','success','profile_rejection']) {
    const storage=new Map(mode==='guest'?[]:[['token','test-member-token']]);let settings=0,profiles=0;
    const member={getSettings:async()=>{settings++;if(mode==='settings_rejection')throw null;return mode==='settings_null'?null:{name:'仅测试账号'}},
      getProfile:async()=>{profiles++;if(mode==='profile_rejection')throw null;return {nickname:'仅测试会员',mobile:'13800000001',credit:13,level:0,avatar:''}}};
    const sandbox={exports:{},uni:{getStorageSync:k=>storage.get(k),setStorageSync:(k,v)=>storage.set(k,v)},
      require:name=>{if(name==='@/pages/libs/ai-balance-api.uts')return {AIBalanceApi:{read:async()=>({code:0,data:{availablePoints:23,frozenPoints:7}})}};if(name==='vue')return vue;if(name==='@dcloudio/uni-app')return {onShow:()=>{}};if(name==='@/api/member/member.uts')return {api:member};throw new Error('未声明会员页测试依赖：'+name)}};
    vm.runInNewContext(memberOutput.outputText,sandbox,{filename:memberFile,timeout:2000});
    const state=sandbox.exports.default.setup({},{expose:()=>{}});state.name.value='旧账号名称';
    await assert.doesNotReject(()=>state.refreshProfile(),mode+'：设置失败不能中断会员资料刷新');
    assert.equal(settings,mode==='guest'?0:1);assert.equal(profiles,mode==='guest'?0:1);
    assert.equal(state.profileLoggedIn.value,mode!=='guest','展示登录状态必须响应本次刷新');
    assert.equal(state.name.value,['guest','settings_rejection','settings_null'].includes(mode)?'':'仅测试账号');
    assert.equal(state.nickname.value,mode==='guest'?'登录 / 注册':mode==='profile_rejection'?'已登录':'仅测试会员');
    if(mode!=='guest'){assert.equal(state.credit.value,'23');assert.equal(state.frozenCredit.value,'7');assert.equal(storage.has('credit'),false)}
    console.log('通过：原会员页脚本 '+mode);
  }
  for(const mode of ['settings_late','profile_late','profile_failure_cache']) {
    const storage=new Map([['token','member-one'],['nickname','旧会员缓存'],['mobile','13800000001'],['credit','999'],['level','9']]);
    let release;const delayed=new Promise(resolve=>{release=resolve});
    const old={nickname:'旧请求会员',mobile:'13800000001',credit:999,level:9,avatar:'old-avatar'};
    const member={getSettings:async()=>mode==='settings_late'?await delayed:{name:'账号一'},getProfile:async()=>{if(mode==='profile_failure_cache')throw null;return mode==='profile_late'?await delayed:old}};
    const sandbox={exports:{},uni:{getStorageSync:k=>storage.get(k),setStorageSync:(k,v)=>storage.set(k,v)},
      require:name=>{if(name==='@/pages/libs/ai-balance-api.uts')return {AIBalanceApi:{read:async()=>({code:50302,data:null})}};if(name==='vue')return vue;if(name==='@dcloudio/uni-app')return {onShow:()=>{},onHide:()=>{},onUnload:()=>{}};if(name==='@/api/member/member.uts')return {api:member};throw new Error(name)}};
    vm.runInNewContext(memberOutput.outputText,sandbox,{filename:memberFile,timeout:2000});
    const state=sandbox.exports.default.setup({},{expose:()=>{}});
    const pending=state.refreshProfile();await new Promise(setImmediate);
    if(mode!=='profile_failure_cache') {
      storage.delete('token');await state.refreshProfile();
      release(mode==='settings_late'?{name:'旧请求账号'}:old);
    }
    await pending;
    assert.equal(state.nickname.value,mode==='profile_failure_cache'?'已登录':'登录 / 注册',mode+'不得恢复其他身份资料');
    assert.notEqual(state.credit.value,'999',mode+'不得展示无归属缓存积分');
    if(mode!=='profile_failure_cache'){assert.equal(state.name.value,'');assert.equal(state.profileLoggedIn.value,false)}
    assert.equal(storage.get('nickname'),'旧会员缓存','迟到响应不得覆盖缓存');
    console.log('通过：会员身份隔离 '+mode);
  }
  for(const switchAccount of [false,true]) {
    const storage=new Map([['token','member-one']]);let release,reads=0;
    const delayed=new Promise(resolve=>{release=resolve});
    const latest={nickname:'最新会员资料',mobile:'13800000002',credit:7,level:0,avatar:'new-avatar'};
    const member={getSettings:async()=>({name:'当前账号'}),getProfile:async()=>++reads===1?await delayed:latest};
    const sandbox={exports:{},uni:{getStorageSync:k=>storage.get(k),setStorageSync:(k,v)=>storage.set(k,v)},
      require:name=>{if(name==='@/pages/libs/ai-balance-api.uts')return {AIBalanceApi:{read:async()=>({code:0,data:{availablePoints:17,frozenPoints:2}})}};if(name==='vue')return vue;if(name==='@dcloudio/uni-app')return {onShow:()=>{}};if(name==='@/api/member/member.uts')return {api:member};throw new Error(name)}};
    vm.runInNewContext(memberOutput.outputText,sandbox,{filename:memberFile,timeout:2000});
    const state=sandbox.exports.default.setup({},{expose:()=>{}});
    const oldRequest=state.refreshProfile();await new Promise(setImmediate);assert.equal(reads,1,'先确认原资料请求已进入等待');
    if(switchAccount)storage.set('token','member-two');
    await state.refreshProfile();release({...latest,nickname:'迟到旧资料',credit:999});await oldRequest;
    assert.equal(state.nickname.value,'最新会员资料');assert.equal(state.credit.value,'17');
    assert.equal(storage.get('nickname'),'最新会员资料');assert.equal(storage.has('credit'),false);
    console.log('通过：会员资料迟到覆盖保护 '+(switchAccount?'切换账号':'同账号刷新'));
  }
  for(const mode of ['zero','unavailable','network','late_account']) {
    const storage=new Map([['token','one'],['credit','999']]);let release,calls=0;
    const pending=new Promise(resolve=>release=resolve),modals=[];
    const member={getSettings:async()=>({name:'当前账号'}),getProfile:async()=>({nickname:'当前会员',mobile:'',credit:999,level:0,avatar:''})};
    const balance={read:async()=>{calls++;if(mode==='network')throw Error('测试网络失败');if(mode==='unavailable')return {code:50302,data:null};if(mode==='late_account'&&calls===1)return pending;return {code:0,data:{availablePoints:0,frozenPoints:0}}}};
    const sandbox={exports:{},uni:{getStorageSync:k=>storage.get(k),setStorageSync:(k,v)=>storage.set(k,v),showModal:o=>modals.push(o),navigateTo:()=>{throw Error('积分入口不得进入旧资产页')}},require:name=>{if(name==='vue')return vue;if(name==='@dcloudio/uni-app')return {onShow:()=>{}};if(name==='@/api/member/member.uts')return {api:member};if(name==='@/pages/libs/ai-balance-api.uts')return {AIBalanceApi:balance};throw Error(name)}};
    vm.runInNewContext(memberOutput.outputText,sandbox,{filename:memberFile,timeout:2000});const state=sandbox.exports.default.setup({},{expose:()=>{}});
    const first=state.refreshProfile();await new Promise(setImmediate);
    if(mode==='late_account'){assert.equal(calls,1);storage.set('token','two');await state.refreshProfile();release({code:0,data:{availablePoints:888,frozenPoints:777}})}
    await first;
    const expected=['zero','late_account'].includes(mode)?'0':'暂不可用';assert.equal(state.credit.value,expected);assert.equal(state.frozenCredit.value,expected);assert.equal(storage.get('credit'),'999','旧缓存不得混入或覆盖新账本');
    state.goCredit();assert.equal(modals.length,1);assert.ok(modals[0].content.includes('当前可消费积分：'+expected));assert.ok(!modals[0].content.includes('999'));
    console.log('通过：新余额展示 '+mode);
  }
  completed=true;
  console.log('本项仅验原登录及会员页脚本；鉴权/资料为显式替身，原表单与错误密码须另做实际H5验收。');
})().catch(error=>{completed=true;console.error(error);process.exitCode=1});

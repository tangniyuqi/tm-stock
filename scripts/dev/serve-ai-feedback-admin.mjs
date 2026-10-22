// 仅本地验收：编译实际后台组件/API及Element Plus，替换全局会话/传输宿主，不伪造反馈数据。
import { createRequire } from 'node:module'
import { pathToFileURL } from 'node:url'
import path from 'node:path'
const runtime = path.resolve(process.argv[2] || '')
if (!process.argv[2]) throw new Error('需要隔离依赖目录')
const require = createRequire(path.join(runtime, 'package.json'))
const { createServer, build } = await import(pathToFileURL(require.resolve('vite')).href)
const { default: vue } = await import(pathToFileURL(require.resolve('@vitejs/plugin-vue')).href)
const repo = path.resolve(import.meta.dirname, '../..')
const slash = value => value.replaceAll('\\', '/')
const entry = `import {createApp} from 'vue';import ElementPlus from 'element-plus';import 'element-plus/dist/index.css';import Page from ${JSON.stringify('/@fs/' + slash(path.join(repo, 'backend/web/src/view/quant/aiFeedback/index.vue')))};createApp(Page).use(ElementPlus).mount('#app');`
const user = `import {reactive} from 'vue';const store=reactive({token:localStorage.getItem('token')||'',userInfo:{ID:Number(localStorage.getItem('testAdminID')||0)}});window.addEventListener('storage',()=>{store.token=localStorage.getItem('token')||'';store.userInfo.ID=Number(localStorage.getItem('testAdminID')||0)});export const useUserStore=()=>store;`
const request = `export default async function(options){const response=await fetch(options.url+(options.params?'?'+new URLSearchParams(options.params):''),{method:options.method.toUpperCase(),headers:{'x-token':localStorage.getItem('token')||'','Content-Type':'application/json'},body:options.data?JSON.stringify(options.data):undefined,signal:AbortSignal.timeout(options.timeout)});let data=await response.json();if(!response.ok)throw {response:{status:response.status,data}};return data;}`
const virtual = {
  name: 'test-only-feedback-host',
  resolveId(id) { if (['\0feedback-user','\0feedback-request'].includes(id)) return id; if (id === 'virtual:feedback-entry') return '\0feedback-entry'; if (id === '@/pinia/modules/user') return '\0feedback-user'; if (id === '@/utils/request') return '\0feedback-request' },
  load(id) { if (id === '\0feedback-entry') return entry; if (id === '\0feedback-user') return user; if (id === '\0feedback-request') return request },
  configureServer(server) { server.middlewares.use((req, res, next) => { if (req.url !== '/') return next();res.setHeader('Content-Type','text/html; charset=utf-8');res.end(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>仅测试：后台反馈处理</title><style>body{margin:0;background:#f4f5f7;font-family:Arial,sans-serif}.test-banner{padding:12px;background:#fff5d8}.gva-search-box,.gva-table-box{margin:16px;padding:20px;background:white;border-radius:8px}</style><div class="test-banner">仅测试：实际后台反馈组件与接口联调，非生产后台，不含真实调用</div><div id="app"></div><script type="module" src="/@id/virtual:feedback-entry"></script></html>`) }) }
}
const cfg = { configFile:false, root:runtime, plugins:[virtual,vue()], resolve:{alias:[{find:/^vue$/,replacement:require.resolve('vue/dist/vue.runtime.esm-bundler.js')},{find:/^element-plus$/,replacement:path.join(runtime,'node_modules/element-plus/es/index.mjs')},{find:/^element-plus\//,replacement:slash(path.join(runtime,'node_modules/element-plus'))+'/'},{find:/^@\//,replacement:slash(path.join(repo,'backend/web/src'))+'/'}]}, server:{host:'127.0.0.1',port:5176,strictPort:true,open:false,hmr:false,fs:{allow:[runtime,path.join(repo,'backend/web/src')]}}, build:{outDir:path.join(runtime,'test-build'),emptyOutDir:true,rolldownOptions:{input:'virtual:feedback-entry'}} }
// 会话/请求模块先由虚拟插件解析，不被通用@路径重写。
cfg.resolve.alias.unshift({find:'@/pinia/modules/user',replacement:'\0feedback-user'},{find:'@/utils/request',replacement:'\0feedback-request'})
if (process.argv.includes('--build-only')) { await build(cfg) } else { const server=await createServer(cfg);await server.listen();console.log('FEEDBACK_UI_READY '+server.resolvedUrls.local[0]) }

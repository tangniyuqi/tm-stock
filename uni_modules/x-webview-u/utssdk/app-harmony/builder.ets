import {WebviewUCustom,tmuiWebViewNode} from "x_webview_u"
@Builder
export function buildView(params: ESObject) {
  tmuiWebViewNode({web:params.web})
}

import { CameraView} from "x_camrea_u"

@Builder
export function buildView(params: ESObject) {
  Column(){
	  CameraView()
  }.width('100%').height('100%')
}

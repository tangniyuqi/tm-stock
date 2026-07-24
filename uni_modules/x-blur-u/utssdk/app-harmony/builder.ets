@Builder
export function buildXdBlurView(params: ESObject) {
  Column(){}.width('100%').height('100%')
  .backgroundColor(params.bgColor)
  .backdropBlur(params.blur*2,{grayscale:[0,0]})
}


@Builder
export function buildXdParentBlurView(params: ESObject) {
  Column(){}.width('100%').height('100%')
  .backgroundColor(params.bgColor)
  .backdropBlur(params.blur*2,{grayscale:[0,0]})
}
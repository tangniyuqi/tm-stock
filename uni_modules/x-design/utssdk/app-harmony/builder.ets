@Builder
export function buildXdBlurView(params: ESObject) {
  Column(){}.width('100%').height('100%')
  .backgroundColor('rgba(255,255,255,0.3)')
  .backdropBlur(params.blur,{grayscale:[50,50]})
}


@Builder
export function buildXdParentBlurView(params: ESObject) {
  Column(){}.width('100%').height('100%')
  .backgroundColor('rgba(255,255,255,0.3)')
  .backdropBlur(params.blur,{grayscale:[50,50]})
}
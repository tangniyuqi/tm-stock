import UIKit
import ObjectiveC

@objc class XblurUHelper: NSObject {

    private static let kBlurTag = 0x7B10
    private static let kColorTag = 0x7B11
    // 关联对象 key
    private static var animatorKey: UInt8 = 0

    /// 应用 / 更新背景模糊（首次创建动画器）
    /// - Parameters:
    ///   - view: 目标 UIView
    ///   - blurFraction: 0~1，精确控制模糊强度
    ///   - backgroundColor: 颜色 tint（含 alpha）
    @objc static func applyBlur(
        to view: UIView,
        blurFraction: CGFloat,
        backgroundColor: UIColor
    ) {
        // 已有动画器：复用，只更新强度和颜色
        if let animator = objc_getAssociatedObject(view, &animatorKey) as? UIViewPropertyAnimator {
            animator.fractionComplete = clampFraction(blurFraction)
            if let colorView = view.viewWithTag(kColorTag) {
                colorView.backgroundColor = backgroundColor
            }
            return
        }

        // 首次安装
        view.subviews
            .filter { $0.tag == kBlurTag || $0.tag == kColorTag }
            .forEach { $0.removeFromSuperview() }

        view.backgroundColor = .clear
        view.clipsToBounds = true

        // 1. 模糊层（effect 初始为 nil，由 animator 控制强度）
        let blurView = UIVisualEffectView(effect: nil)
        blurView.frame = view.bounds
        blurView.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        blurView.isUserInteractionEnabled = false
        blurView.tag = kBlurTag
        view.insertSubview(blurView, at: 0)

        // 2. 通过 UIViewPropertyAnimator 的 fractionComplete 精确控制模糊强度
        let animator = UIViewPropertyAnimator(duration: 1, curve: .linear) {
            blurView.effect = UIBlurEffect(style: .regular)
        }
        animator.pausesOnCompletion = true
        animator.fractionComplete = clampFraction(blurFraction)
        objc_setAssociatedObject(view, &animatorKey, animator, .OBJC_ASSOCIATION_RETAIN_NONATOMIC)

        // 3. 颜色叠加层（含 alpha，等价于 web 的 background-color:rgba(...)）
        let colorView = UIView(frame: view.bounds)
        colorView.backgroundColor = backgroundColor
        colorView.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        colorView.isUserInteractionEnabled = false
        colorView.tag = kColorTag
        view.insertSubview(colorView, aboveSubview: blurView)
    }

    /// 移除背景模糊效果
    @objc static func removeBlur(from view: UIView) {
        if let animator = objc_getAssociatedObject(view, &animatorKey) as? UIViewPropertyAnimator {
            animator.stopAnimation(true)
            objc_setAssociatedObject(view, &animatorKey, nil, .OBJC_ASSOCIATION_RETAIN_NONATOMIC)
        }
        view.subviews
            .filter { $0.tag == kBlurTag || $0.tag == kColorTag }
            .forEach { $0.removeFromSuperview() }
    }

    /// 限制 fraction 在 (0,1) 开区间内，避免 animator 进入终止状态
    private static func clampFraction(_ v: CGFloat) -> CGFloat {
        if v <= 0.001 { return 0.001 }
        if v >= 0.999 { return 0.999 }
        return v
    }
}

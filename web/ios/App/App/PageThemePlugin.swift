import Capacitor
import UIKit

@objc(PageThemePlugin)
public class PageThemePlugin: CAPPlugin, CAPBridgedPlugin {
    public let identifier = "PageThemePlugin"
    public let jsName = "PageTheme"
    public let pluginMethods: [CAPPluginMethod] = [
        CAPPluginMethod(name: "apply", returnType: CAPPluginReturnPromise)
    ]

    @objc func apply(_ call: CAPPluginCall) {
        guard let background = call.getString("background"),
              let isDark = call.getBool("isDark"),
              let color = colorOfHex(background) else {
            call.reject("background (#RRGGBB) and isDark are required")
            return
        }
        DispatchQueue.main.async {
            self.bridge?.viewController?.view.backgroundColor = color
            self.bridge?.webView?.backgroundColor = color
            self.bridge?.webView?.scrollView.backgroundColor = color
            self.bridge?.statusBarStyle = isDark ? .lightContent : .darkContent
            call.resolve()
        }
    }

    private func colorOfHex(_ hex: String) -> UIColor? {
        let digits = hex.hasPrefix("#") ? String(hex.dropFirst()) : hex
        guard digits.count == 6, let value = UInt32(digits, radix: 16) else { return nil }
        return UIColor(
            red: CGFloat((value >> 16) & 0xFF) / 255,
            green: CGFloat((value >> 8) & 0xFF) / 255,
            blue: CGFloat(value & 0xFF) / 255,
            alpha: 1
        )
    }
}

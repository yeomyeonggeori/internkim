import Tauri
import UIKit
import WebKit

class PageTheme: Decodable {
    let background: String
    let isDark: Bool
}

class PageThemePlugin: Plugin {
    private weak var webview: WKWebView?

    @objc public override func load(webview: WKWebView) {
        self.webview = webview
    }

    @objc public func apply(_ invoke: Invoke) throws {
        let theme = try invoke.parseArgs(PageTheme.self)
        guard let color = colorOfHex(theme.background) else {
            invoke.reject("background is not a #RRGGBB colour: \(theme.background)")
            return
        }
        DispatchQueue.main.async {
            self.webview?.backgroundColor = color
            self.webview?.scrollView.backgroundColor = color
            self.webview?.window?.backgroundColor = color
            self.webview?.window?.overrideUserInterfaceStyle = theme.isDark ? .dark : .light
            invoke.resolve()
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

@_cdecl("init_plugin_page_theme")
public func initPlugin() -> Plugin {
    return PageThemePlugin()
}

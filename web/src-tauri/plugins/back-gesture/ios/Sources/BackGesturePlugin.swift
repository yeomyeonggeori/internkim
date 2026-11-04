import Tauri
import UIKit
import WebKit

class BackGestureChoice: Decodable {
    let isAllowed: Bool
}

class BackGesturePlugin: Plugin {
    private weak var webview: WKWebView?

    @objc public override func load(webview: WKWebView) {
        self.webview = webview
        DispatchQueue.main.async {
            webview.allowsBackForwardNavigationGestures = true
        }
    }

    @objc public func apply(_ invoke: Invoke) throws {
        let choice = try invoke.parseArgs(BackGestureChoice.self)
        DispatchQueue.main.async {
            self.webview?.allowsBackForwardNavigationGestures = choice.isAllowed
            invoke.resolve()
        }
    }
}

@_cdecl("init_plugin_back_gesture")
public func initPlugin() -> Plugin {
    return BackGesturePlugin()
}

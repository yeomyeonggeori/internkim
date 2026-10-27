import Tauri
import UIKit
import WebKit

class BackGesturePlugin: Plugin {
    @objc public override func load(webview: WKWebView) {
        DispatchQueue.main.async {
            webview.allowsBackForwardNavigationGestures = true
        }
    }
}

@_cdecl("init_plugin_back_gesture")
public func initPlugin() -> Plugin {
    return BackGesturePlugin()
}

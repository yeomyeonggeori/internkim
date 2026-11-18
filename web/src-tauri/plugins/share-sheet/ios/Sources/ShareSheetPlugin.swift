import Tauri
import UIKit
import WebKit

class SharedFile: Decodable {
    let path: String
}

class ShareSheetPlugin: Plugin {
    private weak var webview: WKWebView?

    @objc public override func load(webview: WKWebView) {
        self.webview = webview
    }

    @objc public func presentFile(_ invoke: Invoke) throws {
        let file = try invoke.parseArgs(SharedFile.self)
        DispatchQueue.main.async {
            guard let webview = self.webview, let presenter = Self.topmostController(above: webview.window?.rootViewController) else {
                invoke.reject("the share sheet has no window to slide up in")
                return
            }
            let sheet = UIActivityViewController(activityItems: [URL(fileURLWithPath: file.path)], applicationActivities: nil)
            sheet.completionWithItemsHandler = { _, _, _, error in
                if let error = error {
                    invoke.reject("the share sheet failed: \(error.localizedDescription)")
                    return
                }
                invoke.resolve()
            }
            if UIDevice.current.userInterfaceIdiom == .pad, let popover = sheet.popoverPresentationController {
                popover.sourceView = webview
                popover.sourceRect = CGRect(x: webview.bounds.midX, y: webview.bounds.midY, width: 1, height: 1)
                popover.permittedArrowDirections = []
            }
            presenter.present(sheet, animated: true)
        }
    }

    private static func topmostController(above root: UIViewController?) -> UIViewController? {
        var topmost = root
        while let presented = topmost?.presentedViewController {
            topmost = presented
        }
        return topmost
    }
}

@_cdecl("init_plugin_share_sheet")
public func initPlugin() -> Plugin {
    return ShareSheetPlugin()
}

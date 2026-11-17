import Tauri
import UIKit
import WebKit

class PrintablePage: Decodable {
    let title: String
    let html: String
}

class PrintSheetPlugin: Plugin {
    private weak var webview: WKWebView?

    @objc public override func load(webview: WKWebView) {
        self.webview = webview
    }

    @objc public func present(_ invoke: Invoke) throws {
        let page = try invoke.parseArgs(PrintablePage.self)
        DispatchQueue.main.async {
            let printInfo = UIPrintInfo(dictionary: nil)
            printInfo.outputType = .general
            printInfo.jobName = page.title

            let controller = UIPrintInteractionController.shared
            controller.printInfo = printInfo
            controller.printFormatter = UIMarkupTextPrintFormatter(markupText: page.html)

            let finished: UIPrintInteractionController.CompletionHandler = { _, _, error in
                if let error = error {
                    invoke.reject("the print sheet failed: \(error.localizedDescription)")
                    return
                }
                invoke.resolve()
            }

            if UIDevice.current.userInterfaceIdiom == .pad, let webview = self.webview {
                let center = CGRect(x: webview.bounds.midX, y: webview.bounds.midY, width: 1, height: 1)
                controller.present(from: center, in: webview, animated: true, completionHandler: finished)
                return
            }
            controller.present(animated: true, completionHandler: finished)
        }
    }
}

@_cdecl("init_plugin_print_sheet")
public func initPlugin() -> Plugin {
    return PrintSheetPlugin()
}

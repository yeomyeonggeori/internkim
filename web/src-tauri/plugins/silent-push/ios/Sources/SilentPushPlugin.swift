import Tauri
import UIKit
import WebKit

class SilentPushPlugin: Plugin {
    @objc public override func load(webview: WKWebView) {
        DispatchQueue.main.async {
            SilentPushForwarding.install()
        }
    }
}

enum SilentPushForwarding {
    private typealias Completion = @convention(block) (UIBackgroundFetchResult) -> Void
    private typealias Handler = @convention(c) (AnyObject, Selector, UIApplication, NSDictionary, Completion) -> Void
    private typealias HandlerBlock = @convention(block) (AnyObject, UIApplication, NSDictionary, @escaping Completion) -> Void

    private static var isInstalled = false
    private static let selector = #selector(UIApplicationDelegate.application(_:didReceiveRemoteNotification:fetchCompletionHandler:))

    static func install() {
        guard !isInstalled, let delegate = UIApplication.shared.delegate else { return }
        isInstalled = true
        let delegateClass: AnyClass = type(of: delegate)
        let existing = class_getInstanceMethod(delegateClass, selector)
        let previous = existing.map { unsafeBitCast(method_getImplementation($0), to: Handler.self) }
        let handler: HandlerBlock = { receiver, application, userInfo, completion in
            Task { @MainActor in
                let ours = await SilentPush.handle(userInfo as? [AnyHashable: Any] ?? [:])
                guard let previous else {
                    completion(ours)
                    return
                }
                previous(receiver, selector, application, userInfo) { theirs in
                    completion(combined(ours, theirs))
                }
            }
        }
        let implementation = imp_implementationWithBlock(handler)
        if let existing {
            method_setImplementation(existing, implementation)
        } else {
            class_addMethod(delegateClass, selector, implementation, "v@:@@@?")
        }
    }

    private static func combined(_ ours: UIBackgroundFetchResult, _ theirs: UIBackgroundFetchResult) -> UIBackgroundFetchResult {
        if ours == .newData || theirs == .newData { return .newData }
        if ours == .failed || theirs == .failed { return .failed }
        return .noData
    }
}

@_cdecl("init_plugin_silent_push")
public func initPlugin() -> Plugin {
    return SilentPushPlugin()
}

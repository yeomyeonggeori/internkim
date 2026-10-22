import Tauri
import UIKit
import UserNotifications
import WebKit

class HoldBackArgs: Decodable {
    let tag: String
}

class ForegroundNotificationsPlugin: Plugin, UNUserNotificationCenterDelegate {
    private weak var wrappedDelegate: UNUserNotificationCenterDelegate?
    private var heldBackTag = ""

    @objc public override func load(webview: WKWebView) {
        let center = UNUserNotificationCenter.current()
        guard center.delegate !== self else { return }
        wrappedDelegate = center.delegate
        center.delegate = self
    }

    @objc public func holdBack(_ invoke: Invoke) throws {
        let args = try invoke.parseArgs(HoldBackArgs.self)
        DispatchQueue.main.async {
            self.heldBackTag = args.tag
            invoke.resolve()
        }
    }

    public func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        let isPush = notification.request.trigger is UNPushNotificationTrigger
        let tag = notification.request.content.threadIdentifier
        let isHeldBack = !heldBackTag.isEmpty && tag == heldBackTag
        guard let wrapped = wrappedDelegate,
              wrapped.responds(to: #selector(UNUserNotificationCenterDelegate.userNotificationCenter(_:willPresent:withCompletionHandler:)))
        else {
            completionHandler(isPush && !isHeldBack ? [.badge, .sound, .banner, .list] : [])
            return
        }
        wrapped.userNotificationCenter?(center, willPresent: notification) { wrappedOptions in
            guard isPush else {
                completionHandler(wrappedOptions)
                return
            }
            completionHandler(isHeldBack ? [] : [.badge, .sound, .banner, .list])
        }
    }

    public func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        guard let wrapped = wrappedDelegate,
              wrapped.responds(to: #selector(UNUserNotificationCenterDelegate.userNotificationCenter(_:didReceive:withCompletionHandler:)))
        else {
            completionHandler()
            return
        }
        wrapped.userNotificationCenter?(center, didReceive: response, withCompletionHandler: completionHandler)
    }
}

@_cdecl("init_plugin_foreground_notifications")
public func initPlugin() -> Plugin {
    return ForegroundNotificationsPlugin()
}

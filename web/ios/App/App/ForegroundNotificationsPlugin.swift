import Capacitor
import UserNotifications

@objc(ForegroundNotificationsPlugin)
public class ForegroundNotificationsPlugin: CAPPlugin, CAPBridgedPlugin, NotificationHandlerProtocol {
    public let identifier = "ForegroundNotificationsPlugin"
    public let jsName = "ForegroundNotifications"
    public let pluginMethods: [CAPPluginMethod] = [
        CAPPluginMethod(name: "holdBack", returnType: CAPPluginReturnPromise)
    ]

    private var pushHandler: NotificationHandlerProtocol?
    private var heldBackTag = ""

    override public func load() {
        guard let router = bridge?.notificationRouter else { return }
        pushHandler = router.pushNotificationHandler
        if pushHandler == nil {
            CAPLog.print("ForegroundNotifications found no push handler to wrap; notifications in the foreground follow iOS defaults")
        }
        router.pushNotificationHandler = self
    }

    @objc func holdBack(_ call: CAPPluginCall) {
        let tag = call.getString("tag") ?? ""
        DispatchQueue.main.async {
            self.heldBackTag = tag
            call.resolve()
        }
    }

    public func willPresent(notification: UNNotification) -> UNNotificationPresentationOptions {
        let shown = pushHandler?.willPresent(notification: notification) ?? []
        let tag = notification.request.content.threadIdentifier
        if !heldBackTag.isEmpty && tag == heldBackTag {
            return []
        }
        return shown
    }

    public func didReceive(response: UNNotificationResponse) {
        pushHandler?.didReceive(response: response)
    }
}

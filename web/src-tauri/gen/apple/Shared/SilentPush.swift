import UIKit
import UserNotifications
import WidgetKit

enum SilentPush {
    static func handle(_ userInfo: [AnyHashable: Any]) async -> UIBackgroundFetchResult {
        if let withdrawn = userInfo["withdrawnMessageIDs"] as? [String] {
            UNUserNotificationCenter.current().removeDeliveredNotifications(withIdentifiers: withdrawn)
            return .newData
        }
        if let readTag = userInfo["readTag"] as? String, !readTag.isEmpty {
            await clearDelivered(threadIdentifier: readTag)
            return .newData
        }
        guard userInfo["widget"] as? String == "attendance" else { return .noData }
        AttendanceWidgetStore.noteServerChange()
        WidgetCenter.shared.reloadAllTimelines()
        if userInfo["clock"] as? String == "clock_out" {
            await AttendanceLockScreenCards.close()
        }
        return .newData
    }

    private static func clearDelivered(threadIdentifier: String) async {
        let center = UNUserNotificationCenter.current()
        let identifiers = await center.deliveredNotifications()
            .filter { $0.request.content.threadIdentifier == threadIdentifier }
            .map { $0.request.identifier }
        center.removeDeliveredNotifications(withIdentifiers: identifiers)
    }
}

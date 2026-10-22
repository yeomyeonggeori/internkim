import UIKit
import UserNotifications
import WidgetKit

enum SilentPush {
    static func handle(_ userInfo: [AnyHashable: Any]) async -> UIBackgroundFetchResult {
        if let withdrawn = userInfo["withdrawnMessageIDs"] as? [String] {
            UNUserNotificationCenter.current().removeDeliveredNotifications(withIdentifiers: withdrawn)
            return .newData
        }
        guard userInfo["widget"] as? String == "attendance" else { return .noData }
        WidgetCenter.shared.reloadAllTimelines()
        if userInfo["clock"] as? String == "clock_out" {
            await AttendanceLockScreenCards.close()
        }
        return .newData
    }
}

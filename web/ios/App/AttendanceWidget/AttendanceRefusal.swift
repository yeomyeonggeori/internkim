import Foundation

enum AttendanceRefusal {
    private static let messageKey = "widget.refusal.message"
    private static let momentKey = "widget.refusal.moment"
    static let shownFor: TimeInterval = 15

    static func keep(_ message: String, at moment: Date = Date()) {
        AttendanceWidgetStore.sharedDefaults?.set(message, forKey: messageKey)
        AttendanceWidgetStore.sharedDefaults?.set(moment.timeIntervalSince1970, forKey: momentKey)
    }

    static func recent(at moment: Date = Date()) -> String? {
        guard let message = AttendanceWidgetStore.sharedDefaults?.string(forKey: messageKey) else { return nil }
        let kept = Date(timeIntervalSince1970: AttendanceWidgetStore.sharedDefaults?.double(forKey: momentKey) ?? 0)
        return moment.timeIntervalSince(kept) < shownFor ? message : nil
    }
}

import Foundation

enum AttendanceLocationChoice {
    private static let openedKey = "widget.locations.opened"
    static let shownFor: TimeInterval = 20

    static func open(at moment: Date = Date()) {
        AttendanceWidgetStore.sharedDefaults?.set(moment.timeIntervalSince1970, forKey: openedKey)
    }

    static func close() {
        AttendanceWidgetStore.sharedDefaults?.removeObject(forKey: openedKey)
    }

    static func closes(after moment: Date = Date()) -> Date? {
        let opened = AttendanceWidgetStore.sharedDefaults?.double(forKey: openedKey) ?? 0
        guard opened > 0 else { return nil }
        let closing = Date(timeIntervalSince1970: opened).addingTimeInterval(shownFor)
        return closing > moment ? closing : nil
    }
}

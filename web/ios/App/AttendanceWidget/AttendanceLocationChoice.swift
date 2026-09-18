import Foundation

enum AttendanceLocationChoice {
    private static let openedKey = "widget.locations.opened"
    static let shownFor: TimeInterval = 20

    static func open(at moment: Date = Date()) {
        UserDefaults.standard.set(moment.timeIntervalSince1970, forKey: openedKey)
    }

    static func close() {
        UserDefaults.standard.removeObject(forKey: openedKey)
    }

    static func closes(after moment: Date = Date()) -> Date? {
        let opened = UserDefaults.standard.double(forKey: openedKey)
        guard opened > 0 else { return nil }
        let closing = Date(timeIntervalSince1970: opened).addingTimeInterval(shownFor)
        return closing > moment ? closing : nil
    }
}

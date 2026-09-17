import Foundation

struct AttendanceToday {
    var workedMinutes: Int = 0
    var isWorking: Bool = false
    var openedAt: Date?
    var clockInTime: String?
    var clockOutTime: String?
    var location: String?

    static func of(rows: [AttendanceRow], now: Date) -> AttendanceToday {
        let ordered = rows.sorted { $0.occurredAt < $1.occurredAt }
        var today = AttendanceToday()
        var openedAt: Date?
        var openedLocation: String?

        for row in ordered {
            guard let moment = momentOf(row.occurredAt), moment <= now else { continue }
            if row.kind == "clock_in" {
                openedAt = moment
                openedLocation = row.location
                if today.clockInTime == nil { today.clockInTime = row.time }
                today.clockOutTime = nil
            } else if row.kind == "clock_out", let opened = openedAt {
                today.workedMinutes += minutesBetween(opened, moment)
                today.clockOutTime = row.time
                openedAt = nil
                openedLocation = nil
            }
        }

        if let opened = openedAt {
            today.workedMinutes += minutesBetween(opened, now)
            today.isWorking = true
            today.openedAt = opened
            today.location = openedLocation
        }
        return today
    }

    private static func minutesBetween(_ from: Date, _ to: Date) -> Int {
        max(0, Int(to.timeIntervalSince(from) / 60))
    }

    private static func momentOf(_ written: String) -> Date? {
        let withFraction = ISO8601DateFormatter()
        withFraction.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let moment = withFraction.date(from: written) { return moment }
        let plain = ISO8601DateFormatter()
        plain.formatOptions = [.withInternetDateTime]
        return plain.date(from: written)
    }
}

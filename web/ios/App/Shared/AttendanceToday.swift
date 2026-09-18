import Foundation

struct AttendanceSegment: Equatable {
    let location: String?
    let startTime: String
    let endTime: String?
    let workedMinutes: Int
    let isOpen: Bool
    let clockInTime: String
    let clockInAt: Date
    let clockOutTime: String?
}

struct AttendanceBar: Equatable {
    let location: String?
    let widthPercent: Double
}

struct AttendanceToday: Equatable {
    var segments: [AttendanceSegment] = []

    private static let minutesPerDay = 1440

    var workedMinutes: Int {
        segments.reduce(0) { $0 + $1.workedMinutes }
    }

    var activeSegment: AttendanceSegment? {
        segments.last { $0.isOpen }
    }

    var isWorking: Bool {
        activeSegment != nil
    }

    var location: String? {
        activeSegment?.location
    }

    var clockInTime: String? {
        segments.last?.clockInTime
    }

    var clockOutTime: String? {
        segments.last { $0.clockOutTime != nil }?.clockOutTime
    }

    func elapsedMinutes(at now: Date) -> Int {
        guard let active = activeSegment else { return workedMinutes }
        let elapsed = now.timeIntervalSince(active.clockInAt) / 60
        return workedMinutes + max(0, Int(elapsed.rounded()))
    }

    func bars(currentTime: String) -> [AttendanceBar] {
        segments.map { segment in
            let end = segment.isOpen ? currentTime : (segment.endTime ?? segment.startTime)
            return AttendanceBar(location: segment.location, widthPercent: Self.dayWidthPercent(segment.startTime, end))
        }
    }

    static func of(rows: [AttendanceRow], today: String, now: Date) -> AttendanceToday {
        let ordered = rows.sorted { $0.occurredAt < $1.occurredAt }
        var segments: [AttendanceSegment] = []
        var open: AttendanceRow?

        for row in ordered {
            if row.kind == "clock_in" {
                if let started = open, !outlivedItsDay(started.occurredAt, row.occurredAt),
                   let segment = closedSegment(on: today, from: started, to: row, clockOutTime: nil) {
                    segments.append(segment)
                }
                open = row
                continue
            }
            guard let started = open else { continue }
            if !outlivedItsDay(started.occurredAt, row.occurredAt),
               let segment = closedSegment(on: today, from: started, to: row, clockOutTime: row.time) {
                segments.append(segment)
            }
            open = nil
        }

        if let started = open, let startedAt = moment(started.occurredAt),
           minutesBetween(startedAt, now) <= minutesPerDay,
           let segment = openSegment(on: today, from: started, startedAt: startedAt) {
            segments.append(segment)
        }
        return AttendanceToday(segments: segments)
    }

    private static func closedSegment(
        on day: String,
        from clockIn: AttendanceRow,
        to end: AttendanceRow,
        clockOutTime: String?
    ) -> AttendanceSegment? {
        guard let startedAt = moment(clockIn.occurredAt) else { return nil }
        if day < clockIn.date || day > end.date { return nil }
        let startTime = day == clockIn.date ? clockIn.time : "00:00"
        let endTime = day == end.date ? end.time : "24:00"
        let worked = localMinutes(endTime) - localMinutes(startTime)
        guard worked > 0 else { return nil }
        return AttendanceSegment(
            location: clockIn.location,
            startTime: startTime,
            endTime: endTime,
            workedMinutes: worked,
            isOpen: false,
            clockInTime: clockIn.time,
            clockInAt: startedAt,
            clockOutTime: clockOutTime
        )
    }

    private static func openSegment(on day: String, from clockIn: AttendanceRow, startedAt: Date) -> AttendanceSegment? {
        if day < clockIn.date { return nil }
        let startTime = day == clockIn.date ? clockIn.time : "00:00"
        return AttendanceSegment(
            location: clockIn.location,
            startTime: startTime,
            endTime: nil,
            workedMinutes: 0,
            isOpen: true,
            clockInTime: clockIn.time,
            clockInAt: startedAt,
            clockOutTime: nil
        )
    }

    private static func outlivedItsDay(_ startedAt: String, _ endedAt: String) -> Bool {
        guard let start = moment(startedAt), let end = moment(endedAt) else { return false }
        return minutesBetween(start, end) > minutesPerDay
    }

    private static func minutesBetween(_ start: Date, _ end: Date) -> Int {
        guard end > start else { return 0 }
        return Int((end.timeIntervalSince(start) / 60).rounded())
    }

    static func dayWidthPercent(_ startTime: String, _ endTime: String) -> Double {
        let start = clamp(localMinutes(startTime))
        let end = clamp(max(start, localMinutes(endTime)))
        return min(100, max(1, Double(end - start) / Double(minutesPerDay) * 100))
    }

    private static func clamp(_ minutes: Int) -> Int {
        min(minutesPerDay, max(0, minutes))
    }

    static func localMinutes(_ localTime: String) -> Int {
        let parts = localTime.split(separator: ":").compactMap { Int($0) }
        let hours = parts.first ?? 0
        let minutes = parts.count > 1 ? parts[1] : 0
        return hours * 60 + minutes
    }

    static func moment(_ written: String) -> Date? {
        let withFraction = ISO8601DateFormatter()
        withFraction.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let found = withFraction.date(from: written) { return found }
        let plain = ISO8601DateFormatter()
        plain.formatOptions = [.withInternetDateTime]
        return plain.date(from: written)
    }
}

enum CompanyClock {
    static func day(of moment: Date, in timeZone: TimeZone) -> String {
        formatted(moment, "yyyy-MM-dd", timeZone)
    }

    static func time(of moment: Date, in timeZone: TimeZone) -> String {
        formatted(moment, "HH:mm", timeZone)
    }

    private static func formatted(_ moment: Date, _ pattern: String, _ timeZone: TimeZone) -> String {
        let formatter = DateFormatter()
        formatter.calendar = Calendar(identifier: .iso8601)
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = timeZone
        formatter.dateFormat = pattern
        return formatter.string(from: moment)
    }
}

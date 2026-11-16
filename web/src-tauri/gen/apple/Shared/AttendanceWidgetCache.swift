import Foundation

struct AttendanceWidgetCache: Codable {
    static let settingsFreshFor: TimeInterval = 6 * 60 * 60
    static let rowsTrustedFor: TimeInterval = 120
    static let tapDrawnFromCacheFor: TimeInterval = 5
    static let tapRedrawnAfter: TimeInterval = 20

    var origin: String
    var settings: CompanySettings?
    var settingsFetchedAt: Date?
    var rows: [AttendanceRow]
    var rowsTrustedUntil: Date?
    var pending: AttendanceRow?
    var rowsListedAt: Date? = nil
    var tappedAt: Date? = nil

    private static let key = "widget.attendanceCache"

    static var sharedStore: UserDefaults? {
        UserDefaults(suiteName: AttendanceWidgetStore.appGroup)
    }

    static func empty(origin: String) -> AttendanceWidgetCache {
        AttendanceWidgetCache(
            origin: origin,
            settings: nil,
            settingsFetchedAt: nil,
            rows: [],
            rowsTrustedUntil: nil,
            pending: nil
        )
    }

    static func resolve(decoded: AttendanceWidgetCache?, origin: String) -> AttendanceWidgetCache {
        guard let decoded, decoded.origin == origin else { return empty(origin: origin) }
        return decoded
    }

    static func held(origin: String, in store: UserDefaults? = sharedStore) -> AttendanceWidgetCache {
        let decoded = store?.data(forKey: key).flatMap { try? JSONDecoder().decode(AttendanceWidgetCache.self, from: $0) }
        return resolve(decoded: decoded, origin: origin)
    }

    static func amend(
        origin: String,
        in store: UserDefaults? = sharedStore,
        _ change: (inout AttendanceWidgetCache) -> Void
    ) {
        var cache = held(origin: origin, in: store)
        change(&cache)
        guard let encoded = try? JSONEncoder().encode(cache) else { return }
        store?.set(encoded, forKey: key)
    }

    static func markTap(at moment: Date = Date()) {
        guard let origin = AttendanceWidgetStore.credential()?.origin else { return }
        amend(origin: origin) { $0.tappedAt = moment }
    }

    static func drawsTapFromCache(tappedAt: Date?, now: Date) -> Bool {
        guard let tappedAt else { return false }
        return now.timeIntervalSince(tappedAt) < tapDrawnFromCacheFor
    }

    static func settingsAreFresh(fetchedAt: Date?, now: Date) -> Bool {
        guard let fetchedAt else { return false }
        return now.timeIntervalSince(fetchedAt) < settingsFreshFor
    }

    static func rowsShown(
        cachedRows: [AttendanceRow],
        pending: AttendanceRow?,
        rowsTrustedUntil: Date?,
        drawsTap: Bool,
        now: Date
    ) -> [AttendanceRow]? {
        let isTrusted = rowsTrustedUntil.map { now < $0 } ?? false
        guard isTrusted || drawsTap else { return nil }
        return pending.map { cachedRows + [$0] } ?? cachedRows
    }

    mutating func settle(pressed: AttendanceRow?, written: AttendanceWrite, added: AttendanceRow?, now: Date) {
        letGo(of: pressed)
        if written.status == "removed", let takenBack = written.eventID {
            rows = rows.filter { $0.eventID != takenBack }
            rowsTrustedUntil = now.addingTimeInterval(Self.rowsTrustedFor)
        } else if let added {
            rows = Self.appending(added, to: rows)
            rowsTrustedUntil = now.addingTimeInterval(Self.rowsTrustedFor)
        } else {
            rowsTrustedUntil = nil
        }
    }

    mutating func refuse(pressed: AttendanceRow?) {
        letGo(of: pressed)
        rowsTrustedUntil = nil
    }

    private mutating func letGo(of pressed: AttendanceRow?) {
        guard let pending, let pressed,
              pending.kind == pressed.kind, pending.occurredAt == pressed.occurredAt else { return }
        self.pending = nil
    }

    static func appending(_ row: AttendanceRow, to rows: [AttendanceRow]) -> [AttendanceRow] {
        rows.filter { $0.eventID != row.eventID } + [row]
    }

    static func optimisticRow(kind: String, location: String?, now: Date, timeZone: TimeZone) -> AttendanceRow {
        AttendanceRow(
            eventID: "pending",
            kind: kind,
            date: CompanyClock.day(of: now, in: timeZone),
            time: CompanyClock.time(of: now, in: timeZone),
            occurredAt: isoString(of: now),
            location: kind == "clock_in" ? location : nil
        )
    }

    static func row(from event: AttendanceEvent, timeZone: TimeZone) -> AttendanceRow? {
        guard let occurredMoment = AttendanceToday.moment(event.occurredAt) else { return nil }
        return AttendanceRow(
            eventID: event.id,
            kind: event.kind,
            date: CompanyClock.day(of: occurredMoment, in: timeZone),
            time: CompanyClock.time(of: occurredMoment, in: timeZone),
            occurredAt: event.occurredAt,
            location: event.location
        )
    }

    private static func isoString(of moment: Date) -> String {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime]
        return formatter.string(from: moment)
    }
}

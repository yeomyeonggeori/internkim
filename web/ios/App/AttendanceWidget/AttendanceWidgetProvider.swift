import WidgetKit

struct AttendanceEntry: TimelineEntry {
    let date: Date
    let today: AttendanceToday
    let locations: [WorkLocation]
    let configuredWorkplace: String?
    let timeZone: TimeZone
    let failure: String?
    var refusal: String?
    var isChoosingLocation = false

    var workedMinutes: Int {
        today.elapsedMinutes(at: date)
    }

    var currentTime: String {
        CompanyClock.time(of: date, in: timeZone)
    }

    func at(_ moment: Date, refusal: String? = nil, isChoosingLocation: Bool = false) -> AttendanceEntry {
        AttendanceEntry(
            date: moment,
            today: today,
            locations: locations,
            configuredWorkplace: configuredWorkplace,
            timeZone: timeZone,
            failure: failure,
            refusal: refusal,
            isChoosingLocation: isChoosingLocation
        )
    }

    static func nothingYet(_ date: Date = Date()) -> AttendanceEntry {
        AttendanceEntry(
            date: date,
            today: AttendanceToday(),
            locations: [],
            configuredWorkplace: nil,
            timeZone: .current,
            failure: nil
        )
    }
}

struct AttendanceProvider: AppIntentTimelineProvider {
    private static let minutesDrawnAhead = 60
    private static let whileIdle: TimeInterval = 30 * 60

    func placeholder(in context: Context) -> AttendanceEntry {
        AttendanceEntry.nothingYet()
    }

    func snapshot(for configuration: AttendanceWidgetConfiguration, in context: Context) async -> AttendanceEntry {
        await read(at: Date(), chosen: configuration.workplace?.name).entry
    }

    func timeline(
        for configuration: AttendanceWidgetConfiguration,
        in context: Context
    ) async -> Timeline<AttendanceEntry> {
        let now = Date()
        let drawn = await read(at: now, chosen: configuration.workplace?.name)
        let (entries, refreshAt) = entries(of: drawn.entry, at: now)
        let redrawAt = drawn.isStale ? min(refreshAt, now.addingTimeInterval(AttendanceWidgetCache.tapRedrawnAfter)) : refreshAt
        return Timeline(entries: entries, policy: .after(redrawAt))
    }

    private func entries(of read: AttendanceEntry, at now: Date) -> ([AttendanceEntry], Date) {
        if !read.today.isWorking, let closing = AttendanceLocationChoice.closes(after: now) {
            return ([read.at(now, isChoosingLocation: true), read.at(closing)], now.addingTimeInterval(Self.whileIdle))
        }

        var entries: [AttendanceEntry] = []
        if let refusal = AttendanceRefusal.recent(at: now) {
            entries.append(read.at(now, refusal: refusal))
            entries.append(read.at(now.addingTimeInterval(AttendanceRefusal.shownFor)))
        } else {
            entries.append(read.at(now))
        }

        guard read.today.isWorking else {
            return (entries, now.addingTimeInterval(Self.whileIdle))
        }
        for minute in 1...Self.minutesDrawnAhead {
            entries.append(read.at(now.addingTimeInterval(TimeInterval(minute * 60))))
        }
        return (entries, now.addingTimeInterval(TimeInterval(Self.minutesDrawnAhead * 60)))
    }

    private func read(at now: Date, chosen: String?) async -> (entry: AttendanceEntry, isStale: Bool) {
        do {
            let api = try AttendanceAPI.held()
            let origin = api.credential.origin
            let cache = AttendanceWidgetCache.held(origin: origin)
            let drawsTap = AttendanceWidgetCache.drawsTapFromCache(tappedAt: cache.tappedAt, now: now)
            var isStale = false

            let settings: CompanySettings
            if let cached = cache.settings, AttendanceWidgetCache.settingsAreFresh(fetchedAt: cache.settingsFetchedAt, now: now) {
                settings = cached
            } else if let cached = cache.settings, drawsTap {
                settings = cached
                isStale = true
            } else {
                let fetched = try await api.settings()
                AttendanceWidgetCache.amend(origin: origin) {
                    $0.settings = fetched
                    $0.settingsFetchedAt = now
                }
                settings = fetched
            }
            let zone = settings.companyTimeZone

            let rows: [AttendanceRow]
            if let shown = AttendanceWidgetCache.rowsShown(
                cachedRows: cache.rows,
                pending: cache.pending,
                rowsTrustedUntil: cache.rowsTrustedUntil,
                drawsTap: drawsTap && cache.rowsListedAt != nil,
                now: now
            ) {
                rows = shown
                isStale = isStale || cache.rowsTrustedUntil.map { now >= $0 } ?? true
            } else {
                let listed = try await api.since(yesterdayOf: now, in: zone).attendance
                AttendanceWidgetCache.amend(origin: origin) {
                    $0.rows = listed
                    $0.rowsListedAt = now
                }
                rows = listed
            }

            let today = AttendanceToday.of(rows: rows, today: CompanyClock.day(of: now, in: zone), now: now)
            let entry = AttendanceEntry(
                date: now,
                today: today,
                locations: settings.workLocations,
                configuredWorkplace: chosen,
                timeZone: zone,
                failure: nil
            )
            return (entry, isStale)
        } catch {
            let entry = AttendanceEntry(
                date: now,
                today: AttendanceToday(),
                locations: [],
                configuredWorkplace: chosen,
                timeZone: .current,
                failure: error.localizedDescription
            )
            return (entry, false)
        }
    }
}

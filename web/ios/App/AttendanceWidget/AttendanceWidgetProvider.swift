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
        await read(at: Date(), chosen: configuration.workplace?.name)
    }

    func timeline(
        for configuration: AttendanceWidgetConfiguration,
        in context: Context
    ) async -> Timeline<AttendanceEntry> {
        let now = Date()
        let read = await read(at: now, chosen: configuration.workplace?.name)
        var entries: [AttendanceEntry] = []

        if !read.today.isWorking, let closing = AttendanceLocationChoice.closes(after: now) {
            let choosing = [read.at(now, isChoosingLocation: true), read.at(closing)]
            return Timeline(entries: choosing, policy: .after(now.addingTimeInterval(Self.whileIdle)))
        }

        if let refusal = AttendanceRefusal.recent(at: now) {
            entries.append(read.at(now, refusal: refusal))
            entries.append(read.at(now.addingTimeInterval(AttendanceRefusal.shownFor)))
        } else {
            entries.append(read.at(now))
        }

        guard read.today.isWorking else {
            return Timeline(entries: entries, policy: .after(now.addingTimeInterval(Self.whileIdle)))
        }
        for minute in 1...Self.minutesDrawnAhead {
            entries.append(read.at(now.addingTimeInterval(TimeInterval(minute * 60))))
        }
        let refreshAt = now.addingTimeInterval(TimeInterval(Self.minutesDrawnAhead * 60))
        return Timeline(entries: entries, policy: .after(refreshAt))
    }

    private func read(at now: Date, chosen: String?) async -> AttendanceEntry {
        do {
            let api = try AttendanceAPI.held()
            let settings = try await api.settings()
            let zone = settings.companyTimeZone
            let listed = try await api.since(yesterdayOf: now, in: zone)
            let today = AttendanceToday.of(rows: listed.attendance, today: CompanyClock.day(of: now, in: zone), now: now)
            return AttendanceEntry(
                date: now,
                today: today,
                locations: settings.workLocations,
                configuredWorkplace: chosen,
                timeZone: zone,
                failure: nil
            )
        } catch {
            return AttendanceEntry(
                date: now,
                today: AttendanceToday(),
                locations: [],
                configuredWorkplace: chosen,
                timeZone: .current,
                failure: error.localizedDescription
            )
        }
    }
}

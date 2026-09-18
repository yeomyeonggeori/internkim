import WidgetKit

struct AttendanceEntry: TimelineEntry {
    let date: Date
    let today: AttendanceToday
    let locations: [WorkLocation]
    let timeZone: TimeZone
    let failure: String?
    var refusal: String?

    var workedMinutes: Int {
        today.elapsedMinutes(at: date)
    }

    var currentTime: String {
        CompanyClock.time(of: date, in: timeZone)
    }

    func at(_ moment: Date, refusal: String? = nil) -> AttendanceEntry {
        AttendanceEntry(date: moment, today: today, locations: locations, timeZone: timeZone, failure: failure, refusal: refusal)
    }

    static func nothingYet(_ date: Date = Date()) -> AttendanceEntry {
        AttendanceEntry(date: date, today: AttendanceToday(), locations: [], timeZone: .current, failure: nil)
    }
}

struct AttendanceProvider: TimelineProvider {
    private static let minutesDrawnAhead = 60
    private static let whileIdle: TimeInterval = 30 * 60

    func placeholder(in context: Context) -> AttendanceEntry {
        AttendanceEntry.nothingYet()
    }

    func getSnapshot(in context: Context, completion: @escaping (AttendanceEntry) -> Void) {
        Task { completion(await read(at: Date())) }
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<AttendanceEntry>) -> Void) {
        Task {
            let now = Date()
            let read = await read(at: now)
            var entries: [AttendanceEntry] = []

            if let refusal = AttendanceRefusal.recent(at: now) {
                entries.append(read.at(now, refusal: refusal))
                entries.append(read.at(now.addingTimeInterval(AttendanceRefusal.shownFor)))
            } else {
                entries.append(read.at(now))
            }

            guard read.today.isWorking else {
                completion(Timeline(entries: entries, policy: .after(now.addingTimeInterval(Self.whileIdle))))
                return
            }
            for minute in 1...Self.minutesDrawnAhead {
                entries.append(read.at(now.addingTimeInterval(TimeInterval(minute * 60))))
            }
            let refreshAt = now.addingTimeInterval(TimeInterval(Self.minutesDrawnAhead * 60))
            completion(Timeline(entries: entries, policy: .after(refreshAt)))
        }
    }

    private func read(at now: Date) async -> AttendanceEntry {
        do {
            let api = try AttendanceAPI.held()
            let settings = try await api.settings()
            let zone = settings.companyTimeZone
            let listed = try await api.since(yesterdayOf: now, in: zone)
            let today = AttendanceToday.of(rows: listed.attendance, today: CompanyClock.day(of: now, in: zone), now: now)
            return AttendanceEntry(date: now, today: today, locations: settings.workLocations, timeZone: zone, failure: nil)
        } catch {
            return AttendanceEntry(
                date: now,
                today: AttendanceToday(),
                locations: [],
                timeZone: .current,
                failure: error.localizedDescription
            )
        }
    }
}

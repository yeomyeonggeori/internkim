import WidgetKit

struct AttendanceEntry: TimelineEntry {
    let date: Date
    let today: AttendanceToday
    let locations: [WorkLocation]
    let failure: String?
    var refusal: String?

    static func nothingYet(_ date: Date = Date()) -> AttendanceEntry {
        AttendanceEntry(date: date, today: AttendanceToday(), locations: [], failure: nil)
    }
}

struct AttendanceProvider: TimelineProvider {
    private static let whileWorking: TimeInterval = 5 * 60
    private static let whileIdle: TimeInterval = 30 * 60

    func placeholder(in context: Context) -> AttendanceEntry {
        AttendanceEntry.nothingYet()
    }

    func getSnapshot(in context: Context, completion: @escaping (AttendanceEntry) -> Void) {
        Task { completion(await read()) }
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<AttendanceEntry>) -> Void) {
        Task {
            var entry = await read()
            let after = entry.today.isWorking ? Self.whileWorking : Self.whileIdle
            let refreshAt = Date().addingTimeInterval(after)

            guard let refusal = AttendanceRefusal.recent() else {
                completion(Timeline(entries: [entry], policy: .after(refreshAt)))
                return
            }
            let settled = entry
            entry.refusal = refusal
            let cleared = AttendanceEntry(
                date: Date().addingTimeInterval(AttendanceRefusal.shownFor),
                today: settled.today,
                locations: settled.locations,
                failure: settled.failure
            )
            completion(Timeline(entries: [entry, cleared], policy: .after(refreshAt)))
        }
    }

    private func read() async -> AttendanceEntry {
        do {
            let api = try AttendanceAPI.held()
            async let listed = api.today()
            async let registered = api.locations()
            let today = AttendanceToday.of(rows: try await listed.attendance, now: Date())
            return AttendanceEntry(
                date: Date(),
                today: today,
                locations: try await registered,
                failure: nil
            )
        } catch {
            return AttendanceEntry(
                date: Date(),
                today: AttendanceToday(),
                locations: [],
                failure: error.localizedDescription
            )
        }
    }
}

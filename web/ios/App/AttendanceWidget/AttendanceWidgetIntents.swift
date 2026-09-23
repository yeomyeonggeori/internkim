import AppIntents
import WidgetKit

@available(iOS 17.0, *)
struct ClockInIntent: AppIntent {
    static var title: LocalizedStringResource = "Clock in"
    static var isDiscoverable = false

    @Parameter(title: "Location")
    var location: String?

    init() {}

    init(location: String?) {
        self.location = location
    }

    func perform() async throws -> some IntentResult {
        await clock(kind: "clock_in", location: location)
        return .result()
    }
}

@available(iOS 17.0, *)
struct ClockOutIntent: AppIntent {
    static var title: LocalizedStringResource = "Clock out"
    static var isDiscoverable = false

    init() {}

    func perform() async throws -> some IntentResult {
        await clock(kind: "clock_out", location: nil)
        return .result()
    }
}

@available(iOS 17.0, *)
struct ShowLocationsIntent: AppIntent {
    static var title: LocalizedStringResource = "Choose another location"
    static var isDiscoverable = false

    init() {}

    func perform() async throws -> some IntentResult {
        AttendanceLocationChoice.open()
        WidgetCenter.shared.reloadAllTimelines()
        return .result()
    }
}

@available(iOS 17.0, *)
struct HideLocationsIntent: AppIntent {
    static var title: LocalizedStringResource = "Back to today"
    static var isDiscoverable = false

    init() {}

    func perform() async throws -> some IntentResult {
        AttendanceLocationChoice.close()
        WidgetCenter.shared.reloadAllTimelines()
        return .result()
    }
}

@available(iOS 17.0, *)
private func clock(kind: String, location: String?) async {
    var closeCards = false
    AttendanceLocationChoice.close()

    guard let api = try? AttendanceAPI.held() else {
        AttendanceRefusal.keep(AttendanceAPIFailure.noKey.localizedDescription)
        WidgetCenter.shared.reloadAllTimelines()
        return
    }

    let origin = api.credential.origin
    let zone = AttendanceWidgetCache.held(origin: origin).settings?.companyTimeZone
    let tapped = Date()

    if let zone {
        let optimistic = AttendanceWidgetCache.optimisticRow(kind: kind, location: location, now: tapped, timeZone: zone)
        AttendanceWidgetCache.amend(origin: origin) {
            $0.pending = optimistic
            $0.rowsTrustedUntil = tapped.addingTimeInterval(AttendanceWidgetCache.rowsTrustedFor)
        }
        WidgetCenter.shared.reloadAllTimelines()
    }

    do {
        let written = try await api.clock(kind: kind, location: location)
        let added = zone.flatMap { zone in written.event.flatMap { AttendanceWidgetCache.row(from: $0, timeZone: zone) } }
        AttendanceWidgetCache.amend(origin: origin) {
            $0.pending = nil
            if let added {
                $0.rows = AttendanceWidgetCache.appending(added, to: $0.rows)
                $0.rowsTrustedUntil = Date().addingTimeInterval(AttendanceWidgetCache.rowsTrustedFor)
            } else {
                $0.rowsTrustedUntil = nil
            }
        }
        closeCards = kind == "clock_out"
    } catch {
        AttendanceWidgetCache.amend(origin: origin) {
            $0.pending = nil
            $0.rowsTrustedUntil = nil
        }
        AttendanceRefusal.keep(error.localizedDescription)
    }
    WidgetCenter.shared.reloadAllTimelines()
    if closeCards { await AttendanceLockScreenCards.close() }
}

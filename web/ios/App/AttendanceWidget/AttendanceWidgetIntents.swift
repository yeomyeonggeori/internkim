import ActivityKit
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
private func clock(kind: String, location: String?) async {
    var closeCards = false
    do {
        _ = try await AttendanceAPI.held().clock(kind: kind, location: location)
        closeCards = kind == "clock_out"
    } catch {
        AttendanceRefusal.keep(error.localizedDescription)
    }
    WidgetCenter.shared.reloadAllTimelines()
    if closeCards { await closeLockScreenCards() }
}

private func closeLockScreenCards() async {
    guard #available(iOS 16.2, *) else { return }
    for activity in Activity<AttendanceActivityAttributes>.activities {
        await activity.end(nil, dismissalPolicy: .immediate)
    }
}

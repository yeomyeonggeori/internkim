import AppIntents
import WidgetKit

/// Clocking in and out from the widget itself, without opening the app. iOS 17
/// is where a widget button may do work; below it the widget opens the app.
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
    do {
        _ = try await AttendanceAPI.held().clock(kind: kind, location: location)
    } catch {
        AttendanceRefusal.keep(error.localizedDescription)
    }
    WidgetCenter.shared.reloadAllTimelines()
}

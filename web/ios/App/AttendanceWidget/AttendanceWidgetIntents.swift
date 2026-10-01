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
        AttendanceClockPress.press(kind: "clock_in", location: location)
        return .result()
    }
}

@available(iOS 17.0, *)
struct ClockOutIntent: LiveActivityIntent {
    static var title: LocalizedStringResource = "Clock out"
    static var isDiscoverable = false

    init() {}

    func perform() async throws -> some IntentResult {
        AttendanceClockPress.press(kind: "clock_out", location: nil)
        return .result()
    }
}

@available(iOS 17.0, *)
struct HomeClockOutIntent: AppIntent {
    static var title: LocalizedStringResource = "Clock out"
    static var isDiscoverable = false

    init() {}

    func perform() async throws -> some IntentResult {
        AttendanceClockPress.press(kind: "clock_out", location: nil)
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
        AttendanceWidgetCache.markTap()
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
        AttendanceWidgetCache.markTap()
        WidgetCenter.shared.reloadAllTimelines()
        return .result()
    }
}

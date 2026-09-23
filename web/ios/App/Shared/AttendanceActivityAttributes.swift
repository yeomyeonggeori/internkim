import ActivityKit
import Foundation

@available(iOS 16.1, *)
struct AttendanceActivityAttributes: ActivityAttributes {
    struct ContentState: Codable, Hashable {
        var startedAt: Double
        var earlierMinutes: Double?
        var location: String

        var startedMoment: Date {
            Date(timeIntervalSince1970: startedAt)
        }

        var countingFrom: Date {
            startedMoment.addingTimeInterval(-max(0, earlierMinutes ?? 0) * 60)
        }
    }
}

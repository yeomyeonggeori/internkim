import ActivityKit
import Foundation

struct AttendanceActivityToken {
    let kind: String
    let token: String
    let replaces: String?
}

final class AttendanceActivityTokens {
    private static let startKind = "apns-activity-start"
    private static let activityKind = "apns-activity"
    private static let heldKey = "attendance.activity.tokens"

    private let announce: @MainActor (AttendanceActivityToken) -> Void
    private var isWatching = false
    @MainActor private lazy var held: [String: String] =
        UserDefaults.standard.dictionary(forKey: Self.heldKey) as? [String: String] ?? [:]

    init(announce: @escaping @MainActor (AttendanceActivityToken) -> Void) {
        self.announce = announce
    }

    static var areAllowed: Bool {
        guard #available(iOS 16.2, *) else { return false }
        return ActivityAuthorizationInfo().areActivitiesEnabled
    }

    func watch() {
        guard !isWatching else { return }
        isWatching = true
        if #available(iOS 17.2, *) {
            Task {
                for await token in Activity<AttendanceActivityAttributes>.pushToStartTokenUpdates {
                    await tell(kind: Self.startKind, token: token)
                }
            }
        }
        if #available(iOS 16.2, *) {
            Task {
                for activity in Activity<AttendanceActivityAttributes>.activities {
                    follow(activity)
                }
                for await activity in Activity<AttendanceActivityAttributes>.activityUpdates {
                    follow(activity)
                }
            }
        }
    }

    @MainActor
    func heldTokens() -> [[String: String]] {
        held.map { ["token": $0.key, "kind": $0.value] }
    }

    @available(iOS 16.2, *)
    private func follow(_ activity: Activity<AttendanceActivityAttributes>) {
        Task {
            for await token in activity.pushTokenUpdates {
                await tell(kind: Self.activityKind, token: token)
            }
        }
        Task {
            for await state in activity.activityStateUpdates where state == .ended || state == .dismissed {
                await forget(kind: Self.activityKind)
            }
        }
    }

    @MainActor
    private func forget(kind: String) {
        held = held.filter { $0.value != kind }
        UserDefaults.standard.set(held, forKey: Self.heldKey)
    }

    @MainActor
    private func tell(kind: String, token: Data) {
        let hex = token.map { String(format: "%02x", $0) }.joined()
        let replaced = held.first { $0.value == kind && $0.key != hex }?.key
        if let replaced { held.removeValue(forKey: replaced) }
        held[hex] = kind
        UserDefaults.standard.set(held, forKey: Self.heldKey)
        announce(AttendanceActivityToken(kind: kind, token: hex, replaces: replaced))
    }
}

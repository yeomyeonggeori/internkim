import ActivityKit
import Tauri
import UIKit
import os

class AttendanceActivityPlugin: Plugin {
    private static let startKind = "apns-activity-start"
    private static let activityKind = "apns-activity"
    private static let heldKey = "attendance.activity.tokens"
    private static let log = Logger(subsystem: "app.intern.kim", category: "attendance-activity")

    private var isWatching = false
    @MainActor private lazy var held: [String: String] =
        UserDefaults.standard.dictionary(forKey: Self.heldKey) as? [String: String] ?? [:]

    @objc public func watchTokens(_ invoke: Invoke) {
        guard !isWatching else {
            invoke.resolve()
            return
        }
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
        invoke.resolve()
    }

    @objc public func heldTokens(_ invoke: Invoke) {
        Task { @MainActor in
            invoke.resolve(["tokens": held.map { ["token": $0.key, "kind": $0.value] }])
        }
    }

    @objc public func lockScreenState(_ invoke: Invoke) {
        guard #available(iOS 16.2, *) else {
            invoke.resolve(["allowed": false])
            return
        }
        invoke.resolve(["allowed": ActivityAuthorizationInfo().areActivitiesEnabled])
    }

    @objc public func closeCards(_ invoke: Invoke) {
        Task {
            await AttendanceLockScreenCards.close()
            invoke.resolve()
        }
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
        var carried: JSObject = ["kind": kind, "token": hex]
        if let replaced { carried["replaces"] = replaced }
        do {
            try trigger("token", data: carried)
        } catch {
            Self.log.error("the \(kind, privacy: .public) token did not reach the page: \(error.localizedDescription, privacy: .public)")
        }
    }
}

@_cdecl("init_plugin_attendance_activity")
public func initPlugin() -> Plugin {
    return AttendanceActivityPlugin()
}

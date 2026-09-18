import ActivityKit
import Capacitor

@objc(AttendanceActivityPlugin)
public class AttendanceActivityPlugin: CAPPlugin, CAPBridgedPlugin {
    public let identifier = "AttendanceActivityPlugin"
    public let jsName = "AttendanceActivity"
    public let pluginMethods: [CAPPluginMethod] = [
        CAPPluginMethod(name: "watchTokens", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "heldTokens", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "lockScreenState", returnType: CAPPluginReturnPromise)
    ]

    private static let startKind = "apns-activity-start"
    private static let activityKind = "apns-activity"

    private var isWatching = false
    @MainActor private var held: [String: String] = [:]

    @objc func watchTokens(_ call: CAPPluginCall) {
        guard !isWatching else {
            call.resolve()
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
        call.resolve()
    }

    @objc func heldTokens(_ call: CAPPluginCall) {
        Task { @MainActor in
            call.resolve(["tokens": held.map { ["token": $0.key, "kind": $0.value] }])
        }
    }

    @objc func lockScreenState(_ call: CAPPluginCall) {
        guard #available(iOS 16.2, *) else {
            call.resolve(["allowed": false])
            return
        }
        call.resolve(["allowed": ActivityAuthorizationInfo().areActivitiesEnabled])
    }

    @available(iOS 16.2, *)
    private func follow(_ activity: Activity<AttendanceActivityAttributes>) {
        Task {
            for await token in activity.pushTokenUpdates {
                await tell(kind: Self.activityKind, token: token)
            }
        }
    }

    @MainActor
    private func tell(kind: String, token: Data) {
        let hex = token.map { String(format: "%02x", $0) }.joined()
        let replaced = held.first { $0.value == kind && $0.key != hex }?.key
        if let replaced { held.removeValue(forKey: replaced) }
        held[hex] = kind
        var carried: [String: Any] = ["kind": kind, "token": hex]
        if let replaced { carried["replaces"] = replaced }
        notifyListeners("token", data: carried, retainUntilConsumed: true)
    }
}

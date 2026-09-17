import ActivityKit
import Capacitor

@objc(AttendanceActivityPlugin)
public class AttendanceActivityPlugin: CAPPlugin, CAPBridgedPlugin {
    public let identifier = "AttendanceActivityPlugin"
    public let jsName = "AttendanceActivity"
    public let pluginMethods: [CAPPluginMethod] = [
        CAPPluginMethod(name: "watchTokens", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "heldTokens", returnType: CAPPluginReturnPromise)
    ]

    private static let startKind = "apns-activity-start"
    private static let activityKind = "apns-activity"

    private var isWatching = false
    private var held: [String: String] = [:]

    @objc func watchTokens(_ call: CAPPluginCall) {
        guard !isWatching else {
            call.resolve()
            return
        }
        isWatching = true
        if #available(iOS 17.2, *) {
            Task {
                for await token in Activity<AttendanceActivityAttributes>.pushToStartTokenUpdates {
                    tell(kind: Self.startKind, token: token)
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
        call.resolve(["tokens": held.map { ["token": $0.key, "kind": $0.value] }])
    }

    @available(iOS 16.2, *)
    private func follow(_ activity: Activity<AttendanceActivityAttributes>) {
        Task {
            for await token in activity.pushTokenUpdates {
                tell(kind: Self.activityKind, token: token)
            }
        }
    }

    private func tell(kind: String, token: Data) {
        let hex = token.map { String(format: "%02x", $0) }.joined()
        held[hex] = kind
        notifyListeners("token", data: ["kind": kind, "token": hex], retainUntilConsumed: true)
    }
}

import ActivityKit

enum AttendanceLockScreenCards {
    static func close() async {
        guard #available(iOS 16.2, *) else { return }
        for activity in Activity<AttendanceActivityAttributes>.activities {
            await activity.end(nil, dismissalPolicy: .immediate)
        }
    }
}

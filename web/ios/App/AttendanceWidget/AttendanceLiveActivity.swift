import ActivityKit
import SwiftUI
import WidgetKit

@available(iOS 16.2, *)
struct AttendanceLiveActivity: Widget {
    var body: some WidgetConfiguration {
        ActivityConfiguration(for: AttendanceActivityAttributes.self) { context in
            AttendanceLockScreenView(state: context.state)
                .padding(16)
                .activityBackgroundTint(Color.black.opacity(0.75))
                .activitySystemActionForegroundColor(.white)
        } dynamicIsland: { context in
            DynamicIsland {
                DynamicIslandExpandedRegion(.leading) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Working").font(.subheadline.weight(.medium))
                        Text(clockedInLine(context.state))
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .lineLimit(1)
                    }
                }
                DynamicIslandExpandedRegion(.trailing) {
                    Text(context.state.startedMoment, style: .timer)
                        .font(.title3.monospacedDigit().weight(.medium))
                        .multilineTextAlignment(.trailing)
                        .frame(maxWidth: 110)
                }
                DynamicIslandExpandedRegion(.bottom) {
                    ClockOutButton()
                }
            } compactLeading: {
                Image(systemName: "bolt.fill").foregroundStyle(.purple)
            } compactTrailing: {
                Text(context.state.startedMoment, style: .timer)
                    .monospacedDigit()
                    .frame(maxWidth: 56)
            } minimal: {
                Image(systemName: "bolt.fill").foregroundStyle(.purple)
            }
        }
    }
}

@available(iOS 16.2, *)
private struct AttendanceLockScreenView: View {
    let state: AttendanceActivityAttributes.ContentState

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Label("internkim", systemImage: "bolt.fill")
                    .font(.caption.weight(.medium))
                    .foregroundStyle(.white.opacity(0.7))
                Spacer()
                if !state.location.isEmpty {
                    Label(state.location, systemImage: "mappin.and.ellipse")
                        .font(.caption)
                        .foregroundStyle(.white.opacity(0.7))
                        .lineLimit(1)
                }
            }
            HStack(alignment: .center) {
                VStack(alignment: .leading, spacing: 2) {
                    Text(state.startedMoment, style: .timer)
                        .font(.system(size: 28, weight: .medium, design: .monospaced))
                        .foregroundStyle(.white)
                    Text(clockedInLine(state))
                        .font(.caption.monospacedDigit())
                        .foregroundStyle(.white.opacity(0.6))
                }
                Spacer()
                ClockOutButton()
                    .frame(width: 110)
            }
        }
    }
}

private struct ClockOutButton: View {
    var body: some View {
        if #available(iOS 17.0, *) {
            Button(intent: ClockOutIntent()) {
                Label("Clock out", systemImage: "arrow.right.from.line").frame(maxWidth: .infinity)
            }
            .buttonStyle(.bordered)
            .tint(.white)
        }
    }
}

@available(iOS 16.2, *)
private func clockedInLine(_ state: AttendanceActivityAttributes.ContentState) -> String {
    let time = state.startedMoment.formatted(date: .omitted, time: .shortened)
    return String(localized: "Clocked in at \(time)")
}

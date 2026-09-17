import SwiftUI
import WidgetKit

struct AttendanceWidgetView: View {
    @Environment(\.widgetFamily) private var family
    let entry: AttendanceEntry

    var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            if let failure = entry.failure {
                header
                Spacer(minLength: 0)
                Text(failure)
                    .font(.footnote)
                    .foregroundStyle(.secondary)
                    .lineLimit(3)
                Spacer(minLength: 0)
            } else if family == .systemMedium {
                mediumBody
            } else {
                smallBody
            }
        }
        .widgetBackground()
    }

    private var header: some View {
        Label("Today", systemImage: "bolt")
            .font(.caption.weight(.medium))
            .foregroundStyle(.secondary)
    }

    private var smallBody: some View {
        VStack(alignment: .leading, spacing: 7) {
            header
            WorkedTime(entry: entry, size: 19)
            AttendanceBar(today: entry.today, locations: entry.locations)
            statusLine
            Spacer(minLength: 0)
            actionButtons(showingLocations: false)
        }
    }

    private var mediumBody: some View {
        HStack(alignment: .top, spacing: 10) {
            VStack(alignment: .leading, spacing: 7) {
                header
                WorkedTime(entry: entry, size: 21)
                whereabouts
                AttendanceBar(today: entry.today, locations: entry.locations)
                statusLine
                Spacer(minLength: 0)
            }
            VStack(spacing: 5) {
                actionButtons(showingLocations: true)
            }
            .frame(width: 124)
        }
    }

    @ViewBuilder
    private var whereabouts: some View {
        Group {
            if entry.today.isWorking, let location = entry.today.location {
                Text(location)
            } else if entry.today.isWorking {
                Text("Working")
            } else if let location = defaultLocation {
                Text("Default location · \(location.name)")
            } else {
                Text("No workplace registered")
            }
        }
        .font(.caption)
        .foregroundStyle(.secondary)
        .lineLimit(1)
    }

    @ViewBuilder
    private var statusLine: some View {
        if let refusal = entry.refusal {
            Label(refusal, systemImage: "exclamationmark.circle")
                .font(.caption2)
                .foregroundStyle(.red)
                .lineLimit(2)
        } else {
            marks
        }
    }

    private var marks: some View {
        HStack(spacing: 9) {
            if let clockIn = entry.today.clockInTime {
                Label(clockIn, systemImage: "arrow.right.to.line")
            }
            if let clockOut = entry.today.clockOutTime, !entry.today.isWorking {
                Label(clockOut, systemImage: "arrow.right.from.line")
            }
            if family != .systemMedium, entry.today.isWorking, let location = entry.today.location {
                Label(location, systemImage: "mappin.and.ellipse").lineLimit(1)
            }
        }
        .font(.caption2.monospacedDigit())
        .foregroundStyle(.secondary)
    }

    private var defaultLocation: WorkLocation? {
        entry.locations.first
    }

    @ViewBuilder
    private func actionButtons(showingLocations: Bool) -> some View {
        if #available(iOS 17.0, *) {
            if entry.today.isWorking {
                Button(intent: ClockOutIntent()) {
                    Label("Clock out", systemImage: "arrow.right.from.line").frame(maxWidth: .infinity)
                }
                .buttonStyle(.bordered)
            } else {
                Button(intent: ClockInIntent(location: defaultLocation?.name)) {
                    Group {
                        if showingLocations, let location = defaultLocation {
                            Label("Clock in at \(location.name)", systemImage: "arrow.right.to.line")
                        } else {
                            Label("Clock in", systemImage: "arrow.right.to.line")
                        }
                    }
                    .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)

                if showingLocations {
                    ForEach(entry.locations.dropFirst().prefix(2), id: \.name) { location in
                        Button(intent: ClockInIntent(location: location.name)) {
                            HStack(spacing: 6) {
                                Circle()
                                    .fill(Color(hex: location.color) ?? .secondary)
                                    .frame(width: 7, height: 7)
                                Text(location.name).lineLimit(1)
                            }
                            .frame(maxWidth: .infinity)
                        }
                        .buttonStyle(.bordered)
                    }
                }
            }
        } else {
            Text("Clock in and out from the app")
                .font(.footnote.weight(.medium))
                .frame(maxWidth: .infinity)
                .padding(.vertical, 10)
                .background(.quaternary, in: RoundedRectangle(cornerRadius: 12))
        }
    }
}

private struct WorkedTime: View {
    let entry: AttendanceEntry
    let size: CGFloat

    var body: some View {
        if entry.today.isWorking {
            Text(countingFrom, style: .timer)
                .font(.system(size: size, weight: .medium, design: .monospaced))
                .lineLimit(1)
                .minimumScaleFactor(0.7)
        } else {
            DurationText(minutes: entry.today.workedMinutes, size: size, dimmed: entry.today.workedMinutes == 0)
        }
    }

    private var countingFrom: Date {
        entry.date.addingTimeInterval(-Double(entry.today.workedMinutes) * 60)
    }
}

private struct DurationText: View {
    let minutes: Int
    let size: CGFloat
    let dimmed: Bool

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: 5) {
            part(String(format: "%02d", minutes / 60), unit: "h")
            part(String(format: "%02d", minutes % 60), unit: "m")
        }
        .opacity(dimmed ? 0.35 : 1)
    }

    private func part(_ digits: String, unit: LocalizedStringKey) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 1) {
            if digits.hasPrefix("0"), digits != "00" {
                Text("0").font(.system(size: size, weight: .medium, design: .monospaced)).opacity(0.4)
                Text(String(digits.dropFirst())).font(.system(size: size, weight: .medium, design: .monospaced))
            } else {
                Text(digits).font(.system(size: size, weight: .medium, design: .monospaced))
            }
            Text(unit).font(.system(size: size * 0.8)).opacity(0.6)
        }
    }
}

private struct AttendanceBar: View {
    let today: AttendanceToday
    let locations: [WorkLocation]

    private var filled: Double {
        min(1, Double(today.workedMinutes) / (60 * 24))
    }

    private var tint: Color {
        guard let name = today.location,
              let location = locations.first(where: { $0.name == name }),
              let color = Color(hex: location.color) else { return .accentColor }
        return color
    }

    var body: some View {
        GeometryReader { area in
            ZStack(alignment: .leading) {
                Capsule().fill(.quaternary)
                Capsule().fill(tint).frame(width: area.size.width * filled)
            }
        }
        .frame(height: 6)
    }
}

extension Color {
    init?(hex: String?) {
        guard let hex else { return nil }
        let digits = hex.hasPrefix("#") ? String(hex.dropFirst()) : hex
        guard digits.count == 6, let value = UInt32(digits, radix: 16) else { return nil }
        self.init(
            red: Double((value >> 16) & 0xFF) / 255,
            green: Double((value >> 8) & 0xFF) / 255,
            blue: Double(value & 0xFF) / 255
        )
    }
}

private extension View {
    @ViewBuilder
    func widgetBackground() -> some View {
        if #available(iOS 17.0, *) {
            containerBackground(for: .widget) { Color(uiColor: .systemBackground) }
        } else {
            padding(14).background(Color(uiColor: .systemBackground))
        }
    }
}

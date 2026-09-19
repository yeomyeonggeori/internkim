import SwiftUI
import WidgetKit

struct AttendanceWidgetView: View {
    @Environment(\.widgetFamily) private var family
    let entry: AttendanceEntry

    var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            if let failure = entry.failure {
                Spacer(minLength: 0)
                Text(failure)
                    .font(.footnote)
                    .foregroundStyle(.secondary)
                    .lineLimit(3)
                Spacer(minLength: 0)
            } else if family == .systemMedium {
                if #available(iOS 17.0, *), entry.isChoosingLocation, !otherLocations.isEmpty {
                    AttendanceLocationPicker(locations: otherLocations)
                } else {
                    mediumBody
                }
            } else {
                smallBody
            }
        }
        .widgetBackground()
    }

    private var smallBody: some View {
        VStack(alignment: .leading, spacing: 8) {
            VStack(alignment: .leading, spacing: 8) {
                WorkedTime(entry: entry, size: 22)
                DayBar(entry: entry)
                statusLine
            }
            .frame(maxHeight: .infinity)
            actionButtons(showingLocations: false)
        }
    }

    private var mediumBody: some View {
        VStack(alignment: .leading, spacing: 8) {
            VStack(alignment: .leading, spacing: 9) {
                HStack(alignment: .firstTextBaseline, spacing: 8) {
                    WorkedTime(entry: entry, size: 24)
                    Spacer(minLength: 0)
                    whereabouts
                }
                DayBar(entry: entry)
                statusLine
            }
            .frame(maxHeight: .infinity)
            HStack(spacing: 8) {
                actionButtons(showingLocations: true)
            }
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
            } else if case let .gone(name) = choice {
                Text("\(name) gone · Edit Widget")
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

    private var choice: AttendanceDefaultWorkplace {
        AttendanceDefaultWorkplace.of(configured: entry.configuredWorkplace, among: entry.locations)
    }

    private var defaultLocation: WorkLocation? {
        choice.location
    }

    private var otherLocations: [WorkLocation] {
        AttendanceDefaultWorkplace.others(besides: defaultLocation, among: entry.locations)
    }

    @ViewBuilder
    private func actionButtons(showingLocations: Bool) -> some View {
        if #available(iOS 17.0, *) {
            if entry.today.isWorking {
                Button(intent: ClockOutIntent()) {
                    Label("Clock out", systemImage: "arrow.right.from.line").frame(maxWidth: .infinity)
                }
                .buttonStyle(.bordered)
            } else if choice.needsChoice {
                if showingLocations, !otherLocations.isEmpty {
                    Button(intent: ShowLocationsIntent()) {
                        HStack(spacing: 4) {
                            Text("Other location").lineLimit(1)
                            Image(systemName: "chevron.right")
                        }
                        .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.bordered)
                } else {
                    Text("Choose a workplace in Edit Widget")
                        .font(.footnote.weight(.medium))
                        .lineLimit(2)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 10)
                        .background(.quaternary, in: RoundedRectangle(cornerRadius: 12))
                }
            } else {
                Button(intent: ClockInIntent(location: defaultLocation?.name)) {
                    Group {
                        if showingLocations, let location = defaultLocation {
                            Label("Clock in at \(location.name)", systemImage: "arrow.right.to.line")
                        } else {
                            Label("Clock in", systemImage: "arrow.right.to.line")
                        }
                    }
                    .lineLimit(1)
                    .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)

                if showingLocations, !otherLocations.isEmpty {
                    Button(intent: ShowLocationsIntent()) {
                        HStack(spacing: 4) {
                            Text("Other location").lineLimit(1)
                            Image(systemName: "chevron.right")
                        }
                        .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.bordered)
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
        DurationText(minutes: entry.workedMinutes, size: size, dimmed: !entry.today.isWorking && entry.workedMinutes == 0)
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

private struct DayBar: View {
    let entry: AttendanceEntry

    private func tint(of bar: AttendanceBar) -> Color {
        guard let name = bar.location,
              let location = entry.locations.first(where: { $0.name == name }),
              let color = Color(hex: location.color) else { return .secondary }
        return color
    }

    var body: some View {
        GeometryReader { area in
            ZStack(alignment: .leading) {
                Capsule().fill(.quaternary)
                HStack(spacing: 0) {
                    ForEach(Array(entry.today.bars(currentTime: entry.currentTime).enumerated()), id: \.offset) { _, bar in
                        Rectangle()
                            .fill(tint(of: bar))
                            .frame(width: area.size.width * min(1, bar.widthPercent / 100))
                    }
                }
                .clipShape(Capsule())
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

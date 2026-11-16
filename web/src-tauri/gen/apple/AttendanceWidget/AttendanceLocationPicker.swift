import SwiftUI

@available(iOS 17.0, *)
struct AttendanceLocationPicker: View {
    let locations: [WorkLocation]

    private static let shown = 6

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("Where are you clocking in?")
                    .font(.caption.weight(.medium))
                    .foregroundStyle(.secondary)
                Spacer(minLength: 0)
                Button(intent: HideLocationsIntent()) {
                    Label("Back", systemImage: "chevron.left").font(.caption)
                }
                .buttonStyle(.plain)
                .foregroundStyle(.secondary)
            }
            Grid(horizontalSpacing: 6, verticalSpacing: 6) {
                ForEach(Array(rows.enumerated()), id: \.offset) { _, row in
                    GridRow {
                        ForEach(row, id: \.name) { location in
                            Button(intent: ClockInIntent(location: location.name)) {
                                name(of: location).frame(maxWidth: .infinity)
                            }
                            .buttonStyle(.bordered)
                        }
                        ForEach(row.count..<perRow, id: \.self) { _ in
                            Color.clear.frame(maxWidth: .infinity, maxHeight: 1)
                        }
                    }
                }
            }
            .frame(maxHeight: .infinity)
        }
    }

    private var offered: [WorkLocation] {
        Array(locations.prefix(Self.shown))
    }

    private var perRow: Int {
        if offered.count <= 2 { return max(1, offered.count) }
        return offered.count <= 4 ? 2 : 3
    }

    private var rows: [[WorkLocation]] {
        stride(from: 0, to: offered.count, by: perRow).map { start in
            Array(offered[start..<min(start + perRow, offered.count)])
        }
    }

    private func name(of location: WorkLocation) -> some View {
        HStack(spacing: 6) {
            Circle()
                .fill(Color(hex: location.color) ?? .secondary)
                .frame(width: 7, height: 7)
            Text(location.name).lineLimit(1)
        }
    }
}

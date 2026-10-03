import SwiftUI

struct AttendanceOtherTimeLink: View {
    let kind: String
    let chosen: AttendanceChosenTime?

    var body: some View {
        if let destination = AttendanceChosenTime.choosingURL(kind: kind) {
            Link(destination: destination) {
                HStack(spacing: 3) {
                    Image(systemName: "clock")
                    Text("Other time")
                    Text(chosen?.time ?? "--:--").monospacedDigit()
                }
                .font(.caption)
                .foregroundStyle(chosen == nil ? AnyShapeStyle(.secondary) : AnyShapeStyle(.tint))
            }
            .frame(maxWidth: .infinity, alignment: .trailing)
        }
    }
}

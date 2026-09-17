import SwiftUI
import WidgetKit

@main
struct AttendanceWidgetBundle: WidgetBundle {
    var body: some Widget {
        AttendanceWidget()
    }
}

struct AttendanceWidget: Widget {
    var body: some WidgetConfiguration {
        StaticConfiguration(kind: "AttendanceWidget", provider: AttendanceProvider()) { entry in
            AttendanceWidgetView(entry: entry)
        }
        .configurationDisplayName("Attendance")
        .description("See today's hours, and clock in and out with a tap.")
        .supportedFamilies([.systemSmall, .systemMedium])
    }
}

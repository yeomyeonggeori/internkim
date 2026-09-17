import SwiftUI
import WidgetKit

@main
struct AttendanceWidgetBundle: WidgetBundle {
    var body: some Widget {
        AttendanceWidget()
        if #available(iOS 16.2, *) {
            AttendanceLiveActivity()
        }
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

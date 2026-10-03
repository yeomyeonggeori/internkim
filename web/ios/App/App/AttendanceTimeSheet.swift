import SwiftUI
import UIKit
import WidgetKit

struct AttendanceTimeSheet: View {
    let kind: String
    let timeZone: TimeZone
    let close: () -> Void
    private let latest = Date()
    @State private var moment: Date

    init(kind: String, timeZone: TimeZone, close: @escaping () -> Void) {
        self.kind = kind
        self.timeZone = timeZone
        self.close = close
        let held = AttendanceChosenTime.held(today: CompanyClock.day(of: Date(), in: timeZone))
        _moment = State(initialValue: held?.moment(in: timeZone) ?? Date())
    }

    var body: some View {
        NavigationView {
            VStack(spacing: 16) {
                DatePicker("", selection: $moment, in: ...latest, displayedComponents: .hourAndMinute)
                    .datePickerStyle(.wheel)
                    .labelsHidden()
                    .environment(\.timeZone, timeZone)
                Button("Clear", role: .destructive) { finish(keeping: nil) }
                Spacer(minLength: 0)
            }
            .padding()
            .navigationTitle(kind == "clock_out" ? "What time did you clock out?" : "What time did you clock in?")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel", action: close)
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { finish(keeping: AttendanceChosenTime.of(moment, in: timeZone)) }
                }
            }
        }
        .navigationViewStyle(.stack)
    }

    private func finish(keeping chosen: AttendanceChosenTime?) {
        if let chosen {
            AttendanceChosenTime.keep(chosen)
        } else {
            AttendanceChosenTime.clear()
        }
        WidgetCenter.shared.reloadAllTimelines()
        close()
    }
}

enum AttendanceTimeSheetOpening {
    @discardableResult
    static func present(from contexts: Set<UIOpenURLContext>, over window: UIWindow?) -> Bool {
        guard let kind = contexts.lazy.compactMap({ AttendanceChosenTime.kind(choosingFrom: $0.url) }).first,
              let root = window?.rootViewController else { return false }
        DispatchQueue.main.async {
            var top = root
            while let presented = top.presentedViewController { top = presented }
            weak var shown: UIViewController?
            let sheet = UIHostingController(rootView: AttendanceTimeSheet(kind: kind, timeZone: companyTimeZone()) {
                shown?.dismiss(animated: true)
            })
            shown = sheet
            sheet.sheetPresentationController?.detents = [.medium()]
            top.present(sheet, animated: true)
        }
        return true
    }

    private static func companyTimeZone() -> TimeZone {
        guard let origin = AttendanceWidgetStore.credential()?.origin else { return .current }
        return AttendanceWidgetCache.held(origin: origin).settings?.companyTimeZone ?? .current
    }
}

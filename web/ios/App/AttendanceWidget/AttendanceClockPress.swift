import Foundation
import WidgetKit

enum AttendanceClockPress {
    static func press(kind: String, location: String?, chosen: AttendanceChosenTime?) {
        AttendanceLocationChoice.close()

        guard let api = try? AttendanceAPI.held() else {
            AttendanceRefusal.keep(AttendanceAPIFailure.noKey.localizedDescription)
            WidgetCenter.shared.reloadAllTimelines()
            return
        }

        let origin = api.credential.origin
        let zone = AttendanceWidgetCache.held(origin: origin).settings?.companyTimeZone
        let tapped = Date()
        let occurred = zone.flatMap { chosen?.moment(in: $0) } ?? tapped
        let optimistic = zone.map { AttendanceWidgetCache.optimisticRow(kind: kind, location: location, now: occurred, timeZone: $0) }

        if let optimistic {
            AttendanceWidgetCache.amend(origin: origin) {
                $0.pending = optimistic
                $0.tappedAt = tapped
                $0.rowsTrustedUntil = tapped.addingTimeInterval(AttendanceWidgetCache.rowsTrustedFor)
            }
            WidgetCenter.shared.reloadAllTimelines()
        }

        let saving = Task {
            await save(api: api, kind: kind, location: location, chosen: chosen, zone: zone, pressed: optimistic)
        }
        ProcessInfo.processInfo.performExpiringActivity(withReason: "Saving an attendance clock press") { expired in
            guard !expired else { return }
            let saved = DispatchSemaphore(value: 0)
            Task {
                await saving.value
                saved.signal()
            }
            saved.wait()
        }
    }

    private static func save(api: AttendanceAPI, kind: String, location: String?, chosen: AttendanceChosenTime?, zone: TimeZone?, pressed: AttendanceRow?) async {
        let origin = api.credential.origin
        var closeCards = false
        do {
            let written = try await api.clock(kind: kind, location: location, at: chosen)
            if chosen != nil { AttendanceChosenTime.clear() }
            let added = zone.flatMap { zone in written.event.flatMap { AttendanceWidgetCache.row(from: $0, timeZone: zone) } }
            AttendanceWidgetCache.amend(origin: origin) {
                $0.settle(pressed: pressed, written: written, added: added, now: Date())
            }
            closeCards = kind == "clock_out"
        } catch {
            AttendanceWidgetCache.amend(origin: origin) {
                $0.refuse(pressed: pressed)
            }
            AttendanceRefusal.keep(error.localizedDescription)
        }
        WidgetCenter.shared.reloadAllTimelines()
        if closeCards { await AttendanceLockScreenCards.close() }
    }
}

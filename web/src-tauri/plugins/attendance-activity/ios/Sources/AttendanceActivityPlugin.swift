import Tauri
import UIKit
import os

class AttendanceActivityPlugin: Plugin {
    private static let log = Logger(subsystem: "app.intern.kim", category: "attendance-activity")

    private lazy var tokens = AttendanceActivityTokens { [weak self] token in
        self?.announce(token)
    }

    @objc public func watchTokens(_ invoke: Invoke) {
        tokens.watch()
        invoke.resolve()
    }

    @objc public func heldTokens(_ invoke: Invoke) {
        Task { @MainActor in
            invoke.resolve(["tokens": tokens.heldTokens()])
        }
    }

    @objc public func lockScreenState(_ invoke: Invoke) {
        invoke.resolve(["allowed": AttendanceActivityTokens.areAllowed])
    }

    @objc public func closeCards(_ invoke: Invoke) {
        Task {
            await AttendanceLockScreenCards.close()
            invoke.resolve()
        }
    }

    @MainActor
    private func announce(_ token: AttendanceActivityToken) {
        var carried: JSObject = ["kind": token.kind, "token": token.token]
        if let replaces = token.replaces { carried["replaces"] = replaces }
        do {
            try trigger("token", data: carried)
        } catch {
            Self.log.error("the \(token.kind, privacy: .public) token did not reach the page: \(error.localizedDescription, privacy: .public)")
        }
    }
}

@_cdecl("init_plugin_attendance_activity")
public func initPlugin() -> Plugin {
    return AttendanceActivityPlugin()
}

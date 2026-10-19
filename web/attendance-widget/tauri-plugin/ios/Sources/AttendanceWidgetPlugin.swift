import Tauri
import WidgetKit

class AttendanceWidgetSupply: Decodable {
    let token: String
    let tokenName: String
    let origin: String
}

class AttendanceWidgetPlugin: Plugin {
    @objc public func install(_ invoke: Invoke) {
        invoke.resolve([
            "installID": AttendanceWidgetStore.installID(),
            "tokenName": AttendanceWidgetStore.heldTokenName()
        ])
    }

    @objc public func supply(_ invoke: Invoke) throws {
        let arguments = try invoke.parseArgs(AttendanceWidgetSupply.self)
        AttendanceWidgetStore.keep(
            AttendanceWidgetCredential(
                token: arguments.token,
                tokenName: arguments.tokenName,
                origin: arguments.origin
            )
        )
        WidgetCenter.shared.reloadAllTimelines()
        invoke.resolve()
    }

    @objc public func forget(_ invoke: Invoke) {
        AttendanceWidgetStore.forget()
        WidgetCenter.shared.reloadAllTimelines()
        invoke.resolve()
    }
}

@_cdecl("init_plugin_attendance_widget")
public func initPlugin() -> Plugin {
    return AttendanceWidgetPlugin()
}

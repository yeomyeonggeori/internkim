import Tauri
import UIKit
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
            "tokenName": AttendanceWidgetStore.credential()?.tokenName ?? ""
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

    @objc public func backgroundRefreshState(_ invoke: Invoke) {
        DispatchQueue.main.async {
            invoke.resolve(["allowed": UIApplication.shared.backgroundRefreshStatus == .available])
        }
    }

    @objc public func openSettings(_ invoke: Invoke) {
        DispatchQueue.main.async {
            guard let settings = URL(string: UIApplication.openSettingsURLString) else {
                invoke.reject("iOS gave no address for this app's Settings page")
                return
            }
            UIApplication.shared.open(settings)
            invoke.resolve()
        }
    }
}

@_cdecl("init_plugin_attendance_widget")
public func initPlugin() -> Plugin {
    return AttendanceWidgetPlugin()
}

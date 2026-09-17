import Capacitor
import WidgetKit

@objc(AttendanceWidgetPlugin)
public class AttendanceWidgetPlugin: CAPPlugin, CAPBridgedPlugin {
    public let identifier = "AttendanceWidgetPlugin"
    public let jsName = "AttendanceWidget"
    public let pluginMethods: [CAPPluginMethod] = [
        CAPPluginMethod(name: "install", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "supply", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "forget", returnType: CAPPluginReturnPromise)
    ]

    @objc func install(_ call: CAPPluginCall) {
        call.resolve([
            "installID": AttendanceWidgetStore.installID(),
            "tokenName": AttendanceWidgetStore.heldTokenName()
        ])
    }

    @objc func supply(_ call: CAPPluginCall) {
        guard let token = call.getString("token"),
              let tokenName = call.getString("tokenName"),
              let origin = call.getString("origin") else {
            call.reject("token, tokenName and origin are required")
            return
        }
        AttendanceWidgetStore.keep(
            AttendanceWidgetCredential(token: token, tokenName: tokenName, origin: origin)
        )
        reloadWidgets()
        call.resolve()
    }

    @objc func forget(_ call: CAPPluginCall) {
        AttendanceWidgetStore.forget()
        reloadWidgets()
        call.resolve()
    }

    private func reloadWidgets() {
        if #available(iOS 14.0, *) {
            WidgetCenter.shared.reloadAllTimelines()
        }
    }
}

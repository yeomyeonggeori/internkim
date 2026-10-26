import Capacitor

@objc(AttendanceActivityPlugin)
public class AttendanceActivityPlugin: CAPPlugin, CAPBridgedPlugin {
    public let identifier = "AttendanceActivityPlugin"
    public let jsName = "AttendanceActivity"
    public let pluginMethods: [CAPPluginMethod] = [
        CAPPluginMethod(name: "watchTokens", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "heldTokens", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "lockScreenState", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "closeCards", returnType: CAPPluginReturnPromise)
    ]

    private lazy var tokens = AttendanceActivityTokens { [weak self] token in
        self?.announce(token)
    }

    @objc func watchTokens(_ call: CAPPluginCall) {
        tokens.watch()
        call.resolve()
    }

    @objc func heldTokens(_ call: CAPPluginCall) {
        Task { @MainActor in
            call.resolve(["tokens": tokens.heldTokens()])
        }
    }

    @objc func lockScreenState(_ call: CAPPluginCall) {
        call.resolve(["allowed": AttendanceActivityTokens.areAllowed])
    }

    @objc func closeCards(_ call: CAPPluginCall) {
        Task {
            await AttendanceLockScreenCards.close()
            call.resolve()
        }
    }

    @MainActor
    private func announce(_ token: AttendanceActivityToken) {
        var carried: [String: Any] = ["kind": token.kind, "token": token.token]
        if let replaces = token.replaces { carried["replaces"] = replaces }
        notifyListeners("token", data: carried, retainUntilConsumed: true)
    }
}

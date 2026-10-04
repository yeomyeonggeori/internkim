import Capacitor
import UIKit

class ViewController: CAPBridgeViewController {
    override open func capacitorDidLoad() {
        webView?.allowsBackForwardNavigationGestures = true
        bridge?.registerPluginInstance(PageThemePlugin())
        bridge?.registerPluginInstance(ForegroundNotificationsPlugin())
        bridge?.registerPluginInstance(AttendanceWidgetPlugin())
        bridge?.registerPluginInstance(AttendanceActivityPlugin())
    }
}

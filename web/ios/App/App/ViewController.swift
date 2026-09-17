import Capacitor
import UIKit

class ViewController: CAPBridgeViewController {
    override open func capacitorDidLoad() {
        bridge?.registerPluginInstance(PageThemePlugin())
        bridge?.registerPluginInstance(ForegroundNotificationsPlugin())
        bridge?.registerPluginInstance(AttendanceWidgetPlugin())
    }
}

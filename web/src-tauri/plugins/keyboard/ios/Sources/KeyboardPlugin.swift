import ObjectiveC
import Tauri
import UIKit
import WebKit

class AccessoryBarChoice: Decodable {
    let isShown: Bool
}

class KeyboardPlugin: Plugin {
    private weak var webview: WKWebView?

    @objc public override func load(webview: WKWebView) {
        self.webview = webview
    }

    @objc public func set_accessory_bar(_ invoke: Invoke) throws {
        let choice = try invoke.parseArgs(AccessoryBarChoice.self)
        DispatchQueue.main.async {
            guard let webview = self.webview else {
                invoke.reject("the keyboard plugin has no web view to change the accessory bar of")
                return
            }
            do {
                try showInputAccessoryBar(choice.isShown, in: webview)
                invoke.resolve()
            } catch {
                invoke.reject(error.localizedDescription)
            }
        }
    }
}

private let contentViewClassPrefix = "WKContent"
private let barlessClassSuffix = "_WithoutInputAccessoryBar"

private struct AccessoryBarError: LocalizedError {
    let errorDescription: String?
}

private func showInputAccessoryBar(_ isShown: Bool, in webview: WKWebView) throws {
    guard let contentView = webview.scrollView.subviews.first(where: {
        NSStringFromClass(type(of: $0)).hasPrefix(contentViewClassPrefix)
    }) else {
        throw AccessoryBarError(errorDescription: "the web view has no \(contentViewClassPrefix) view to change the accessory bar of")
    }
    let currentClass: AnyClass = type(of: contentView)
    let currentName = NSStringFromClass(currentClass)
    let isBarless = currentName.hasSuffix(barlessClassSuffix)
    if isShown == !isBarless { return }
    if isShown {
        guard let originalClass = class_getSuperclass(currentClass) else {
            throw AccessoryBarError(errorDescription: "\(currentName) has no original class to bring the accessory bar back with")
        }
        object_setClass(contentView, originalClass)
    } else {
        guard let barlessClass = barlessSubclass(of: currentClass) else {
            throw AccessoryBarError(errorDescription: "\(currentName) could not be subclassed to hide the accessory bar")
        }
        object_setClass(contentView, barlessClass)
    }
    contentView.reloadInputViews()
}

private func barlessSubclass(of originalClass: AnyClass) -> AnyClass? {
    let name = NSStringFromClass(originalClass) + barlessClassSuffix
    if let existing = NSClassFromString(name) { return existing }
    guard let subclass = objc_allocateClassPair(originalClass, name, 0) else { return nil }
    let selector = #selector(getter: UIResponder.inputAccessoryView)
    guard let method = class_getInstanceMethod(UIResponder.self, selector) else { return nil }
    let noAccessoryView: @convention(block) (AnyObject) -> UIView? = { _ in nil }
    class_addMethod(subclass, selector, imp_implementationWithBlock(noAccessoryView), method_getTypeEncoding(method))
    objc_registerClassPair(subclass)
    return subclass
}

@_cdecl("init_plugin_keyboard")
public func initPlugin() -> Plugin {
    return KeyboardPlugin()
}

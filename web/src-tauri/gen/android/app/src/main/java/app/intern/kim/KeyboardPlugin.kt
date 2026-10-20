package app.intern.kim

import android.app.Activity
import android.view.ViewGroup
import android.webkit.WebView
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import app.tauri.annotation.TauriPlugin
import app.tauri.plugin.JSObject
import app.tauri.plugin.Plugin

@TauriPlugin
class KeyboardPlugin(activity: Activity) : Plugin(activity) {
  private var isOpen = false

  override fun load(webView: WebView) {
    ViewCompat.setOnApplyWindowInsetsListener(webView) { view, insets ->
      val keyboardIsOpen = insets.isVisible(WindowInsetsCompat.Type.ime())
      val keyboardHeight = if (keyboardIsOpen) insets.getInsets(WindowInsetsCompat.Type.ime()).bottom else 0
      val layout = view.layoutParams
      if (layout is ViewGroup.MarginLayoutParams && layout.bottomMargin != keyboardHeight) {
        layout.bottomMargin = keyboardHeight
        view.layoutParams = layout
      }
      if (keyboardIsOpen != isOpen) {
        isOpen = keyboardIsOpen
        trigger("change", JSObject().put("isOpen", keyboardIsOpen))
      }
      ViewCompat.onApplyWindowInsets(view, insets)
    }
  }
}

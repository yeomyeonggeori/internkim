package app.intern.kim

import android.app.Activity
import android.graphics.Color
import androidx.core.view.WindowCompat
import app.tauri.annotation.Command
import app.tauri.annotation.InvokeArg
import app.tauri.annotation.TauriPlugin
import app.tauri.plugin.Invoke
import app.tauri.plugin.Plugin

@InvokeArg
class PageTheme {
  lateinit var background: String
  var isDark: Boolean = false
}

@TauriPlugin
class PageThemePlugin(private val activity: Activity) : Plugin(activity) {
  @Command
  fun apply(invoke: Invoke) {
    val theme = invoke.parseArgs(PageTheme::class.java)
    val color = try {
      Color.parseColor(theme.background)
    } catch (notAColour: IllegalArgumentException) {
      invoke.reject("background is not a colour: ${theme.background}")
      return
    }
    activity.runOnUiThread {
      val window = activity.window
      window.statusBarColor = color
      WindowCompat.getInsetsController(window, window.decorView).isAppearanceLightStatusBars = !theme.isDark
      invoke.resolve()
    }
  }
}

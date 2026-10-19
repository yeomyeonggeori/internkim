package app.intern.kim.attendance

import android.app.Activity
import app.tauri.annotation.Command
import app.tauri.annotation.InvokeArg
import app.tauri.annotation.TauriPlugin
import app.tauri.plugin.Invoke
import app.tauri.plugin.JSObject
import app.tauri.plugin.Plugin

@InvokeArg
class AttendanceWidgetSupply {
  lateinit var token: String
  lateinit var tokenName: String
  lateinit var origin: String
}

@TauriPlugin
class AttendanceWidgetPlugin(private val activity: Activity) : Plugin(activity) {
  @Command
  fun install(invoke: Invoke) {
    val held = JSObject()
    held.put("installID", AttendanceWidgetHandoff.installID(activity))
    held.put("tokenName", AttendanceWidgetHandoff.heldTokenName(activity))
    invoke.resolve(held)
  }

  @Command
  fun supply(invoke: Invoke) {
    val arguments = invoke.parseArgs(AttendanceWidgetSupply::class.java)
    AttendanceWidgetHandoff.supply(activity, arguments.token, arguments.tokenName, arguments.origin)
    invoke.resolve()
  }

  @Command
  fun forget(invoke: Invoke) {
    AttendanceWidgetHandoff.forget(activity)
    invoke.resolve()
  }
}

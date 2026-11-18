package app.intern.kim

import android.app.Activity
import android.content.ActivityNotFoundException
import android.content.ClipData
import android.content.Intent
import androidx.core.content.FileProvider
import app.tauri.annotation.Command
import app.tauri.annotation.InvokeArg
import app.tauri.annotation.TauriPlugin
import app.tauri.plugin.Invoke
import app.tauri.plugin.Plugin
import java.io.File

@InvokeArg
class SharedFile {
  lateinit var path: String
  lateinit var mediaType: String
}

@TauriPlugin
class ShareSheetPlugin(private val activity: Activity) : Plugin(activity) {
  @Command
  fun presentFile(invoke: Invoke) {
    val shared = invoke.parseArgs(SharedFile::class.java)
    val file = File(shared.path)
    val address = try {
      FileProvider.getUriForFile(activity, "${activity.packageName}.fileprovider", file)
    } catch (outsideSharedFolders: IllegalArgumentException) {
      invoke.reject("${file.name} is outside the folders the app shares: ${outsideSharedFolders.message}")
      return
    }
    val sending = Intent(Intent.ACTION_SEND).apply {
      type = shared.mediaType
      putExtra(Intent.EXTRA_STREAM, address)
      clipData = ClipData.newRawUri(file.name, address)
      addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
    }
    activity.runOnUiThread {
      try {
        activity.startActivity(Intent.createChooser(sending, null))
        invoke.resolve()
      } catch (noTaker: ActivityNotFoundException) {
        invoke.reject("no app on this phone can take ${file.name}", noTaker)
      }
    }
  }
}

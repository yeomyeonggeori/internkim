package app.intern.kim

import android.Manifest
import android.app.Activity
import android.content.ContentValues
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.provider.MediaStore
import app.tauri.PermissionState
import app.tauri.annotation.Command
import app.tauri.annotation.InvokeArg
import app.tauri.annotation.Permission
import app.tauri.annotation.PermissionCallback
import app.tauri.annotation.TauriPlugin
import app.tauri.plugin.Invoke
import app.tauri.plugin.Plugin
import java.io.File
import java.io.IOException

private const val STORAGE_PERMISSION = "storage"

@InvokeArg
class GalleryItem {
  lateinit var path: String
  lateinit var mediaType: String
}

@TauriPlugin(permissions = [Permission(strings = [Manifest.permission.WRITE_EXTERNAL_STORAGE], alias = STORAGE_PERMISSION)])
class MediaSavePlugin(private val activity: Activity) : Plugin(activity) {
  private val needsStoragePermission = Build.VERSION.SDK_INT < Build.VERSION_CODES.Q

  @Command
  fun saveIntoGallery(invoke: Invoke) {
    if (needsStoragePermission && getPermissionState(STORAGE_PERMISSION) != PermissionState.GRANTED) {
      requestPermissionForAlias(STORAGE_PERMISSION, invoke, "storagePermissionAnswered")
      return
    }
    save(invoke)
  }

  @PermissionCallback
  private fun storagePermissionAnswered(invoke: Invoke) {
    if (getPermissionState(STORAGE_PERMISSION) != PermissionState.GRANTED) {
      invoke.reject("the app may not add to the photo gallery: storage permission was not granted")
      return
    }
    save(invoke)
  }

  private fun save(invoke: Invoke) {
    val item = invoke.parseArgs(GalleryItem::class.java)
    val file = File(item.path)
    val isVideo = item.mediaType.startsWith("video/")
    val resolver = activity.contentResolver
    val entry = resolver.insert(collectionFor(isVideo), entryValues(file.name, item.mediaType, isVideo))
    if (entry == null) {
      invoke.reject("the photo gallery made no entry for ${file.name}")
      return
    }
    try {
      copyInto(entry, file)
      if (!needsStoragePermission) {
        resolver.update(entry, ContentValues().apply { put(MediaStore.MediaColumns.IS_PENDING, 0) }, null, null)
      }
      invoke.resolve()
    } catch (failure: IOException) {
      resolver.delete(entry, null, null)
      invoke.reject("${file.name} could not be written into the photo gallery: ${failure.message}", failure)
    } catch (failure: SecurityException) {
      resolver.delete(entry, null, null)
      invoke.reject("the photo gallery refused ${file.name}: ${failure.message}", failure)
    }
  }

  private fun collectionFor(isVideo: Boolean): Uri =
    if (isVideo) MediaStore.Video.Media.EXTERNAL_CONTENT_URI else MediaStore.Images.Media.EXTERNAL_CONTENT_URI

  private fun entryValues(name: String, mediaType: String, isVideo: Boolean): ContentValues =
    ContentValues().apply {
      put(MediaStore.MediaColumns.DISPLAY_NAME, name)
      put(MediaStore.MediaColumns.MIME_TYPE, mediaType)
      if (!needsStoragePermission) {
        put(MediaStore.MediaColumns.RELATIVE_PATH, if (isVideo) Environment.DIRECTORY_MOVIES else Environment.DIRECTORY_PICTURES)
        put(MediaStore.MediaColumns.IS_PENDING, 1)
      }
    }

  private fun copyInto(entry: Uri, file: File) {
    val output = activity.contentResolver.openOutputStream(entry) ?: throw IOException("the photo gallery entry $entry could not be opened")
    output.use { destination -> file.inputStream().use { source -> source.copyTo(destination) } }
  }
}

package app.intern.kim

import android.app.Activity
import android.app.DownloadManager
import android.database.Cursor
import android.net.Uri
import android.os.Handler
import android.os.Looper
import android.webkit.WebView
import app.tauri.annotation.Command
import app.tauri.annotation.InvokeArg
import app.tauri.annotation.TauriPlugin
import app.tauri.plugin.Channel
import app.tauri.plugin.Invoke
import app.tauri.plugin.JSObject
import app.tauri.plugin.Plugin
import java.io.File
import java.io.IOException
import java.util.concurrent.Executors

private const val RECEIVING_FOLDER = "file-download"
private const val POLL_INTERVAL_MILLISECONDS = 250L
private const val UNNAMED_MEDIA_TYPE = "application/octet-stream"

@InvokeArg
class DownloadRequest {
  lateinit var address: String
  lateinit var destination: String
  lateinit var onProgress: Channel
}

private class Delivery(val invoke: Invoke, val destination: File, val received: File, val onProgress: Channel)

@TauriPlugin
class FileDownloadPlugin(private val activity: Activity) : Plugin(activity) {
  private val downloads = activity.getSystemService(DownloadManager::class.java)
  private val poller = Handler(Looper.getMainLooper())
  private val copier = Executors.newSingleThreadExecutor()
  private val deliveries = mutableMapOf<Long, Delivery>()

  override fun load(webView: WebView) {
    super.load(webView)
    removeDownloadsOfAnEarlierRun()
  }

  @Command
  fun download(invoke: Invoke) {
    val request = invoke.parseArgs(DownloadRequest::class.java)
    val destination = File(request.destination)
    val receivingFolder = activity.getExternalFilesDir(RECEIVING_FOLDER)
    if (receivingFolder == null) {
      invoke.reject("this phone has no app storage the download manager can write ${destination.name} into")
      return
    }
    val received = File(File(receivingFolder, destination.parentFile?.name ?: "download"), destination.name)
    received.parentFile?.mkdirs()
    val enqueued = try {
      downloads.enqueue(
        DownloadManager.Request(Uri.parse(request.address))
          .setTitle(destination.name)
          .setDestinationUri(Uri.fromFile(received))
      )
    } catch (refused: IllegalArgumentException) {
      invoke.reject("the download manager refused ${request.address}: ${refused.message}", refused)
      return
    }
    poller.post {
      deliveries[enqueued] = Delivery(invoke, destination, received, request.onProgress)
      if (deliveries.size == 1) poller.postDelayed(::poll, POLL_INTERVAL_MILLISECONDS)
    }
  }

  private fun poll() {
    for ((identifier, delivery) in deliveries.toList()) {
      downloads.query(DownloadManager.Query().setFilterById(identifier)).use { row -> follow(identifier, delivery, row) }
    }
    if (deliveries.isNotEmpty()) poller.postDelayed(::poll, POLL_INTERVAL_MILLISECONDS)
  }

  private fun follow(identifier: Long, delivery: Delivery, row: Cursor) {
    if (!row.moveToFirst()) {
      deliveries.remove(identifier)
      delivery.invoke.reject("the download of ${delivery.destination.name} was cancelled")
      return
    }
    when (row.getInt(row.getColumnIndexOrThrow(DownloadManager.COLUMN_STATUS))) {
      DownloadManager.STATUS_SUCCESSFUL -> {
        deliveries.remove(identifier)
        val mediaType = row.getString(row.getColumnIndexOrThrow(DownloadManager.COLUMN_MEDIA_TYPE)) ?: UNNAMED_MEDIA_TYPE
        copier.execute { keep(identifier, delivery, mediaType) }
      }
      DownloadManager.STATUS_FAILED -> {
        deliveries.remove(identifier)
        val reason = row.getInt(row.getColumnIndexOrThrow(DownloadManager.COLUMN_REASON))
        downloads.remove(identifier)
        delivery.invoke.reject("the download of ${delivery.destination.name} failed (download manager reason $reason)")
      }
      else -> report(delivery, row)
    }
  }

  private fun report(delivery: Delivery, row: Cursor) {
    val receivedBytes = row.getLong(row.getColumnIndexOrThrow(DownloadManager.COLUMN_BYTES_DOWNLOADED_SO_FAR))
    val totalBytes = row.getLong(row.getColumnIndexOrThrow(DownloadManager.COLUMN_TOTAL_SIZE_BYTES))
    delivery.onProgress.send(
      JSObject().apply {
        put("receivedBytes", receivedBytes)
        put("totalBytes", maxOf(0L, totalBytes))
      }
    )
  }

  private fun keep(identifier: Long, delivery: Delivery, mediaType: String) {
    try {
      delivery.received.copyTo(delivery.destination, overwrite = true)
      delivery.invoke.resolve(JSObject().apply { put("mediaType", mediaType) })
    } catch (failure: IOException) {
      delivery.invoke.reject("${delivery.destination.name} could not be moved out of the download manager's folder: ${failure.message}", failure)
    } finally {
      downloads.remove(identifier)
    }
  }

  private fun removeDownloadsOfAnEarlierRun() {
    val leftovers = mutableListOf<Long>()
    downloads.query(DownloadManager.Query()).use { rows ->
      val identifierColumn = rows.getColumnIndexOrThrow(DownloadManager.COLUMN_ID)
      while (rows.moveToNext()) leftovers.add(rows.getLong(identifierColumn))
    }
    if (leftovers.isNotEmpty()) downloads.remove(*leftovers.toLongArray())
    activity.getExternalFilesDir(RECEIVING_FOLDER)?.deleteRecursively()
  }
}

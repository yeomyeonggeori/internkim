package app.intern.kim

import android.content.Intent
import app.intern.kim.attendance.AttendanceWidgetPush
import app.intern.kim.notification.SenderNotificationPoster
import app.intern.kim.notification.SenderPicture
import app.intern.kim.notification.SentNotification
import app.tauri.notification.NotificationPlugin
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage

class SenderNotificationService : FirebaseMessagingService() {
  override fun onNewToken(token: String) {
    NotificationPlugin.instance?.handleNewToken(token)
  }

  override fun onMessageReceived(message: RemoteMessage) {
    if (AttendanceWidgetPush.isFor(message.data)) {
      AttendanceWidgetPush.redrawFromServer(this)
      return
    }
    SentNotification.from(message)?.let { sent ->
      SenderNotificationPoster(this, Intent(this, MainActivity::class.java)).post(sent, SenderPicture.of(this, sent.pictureURL))
    }
    NotificationPlugin.instance?.triggerPushMessage(pushData(message))
  }

  private fun pushData(message: RemoteMessage): Map<String, Any> {
    val data = mutableMapOf<String, Any>("data" to message.data, "sentTime" to message.sentTime)
    message.messageId?.let { data["messageId"] = it }
    message.from?.let { data["from"] = it }
    return data
  }
}

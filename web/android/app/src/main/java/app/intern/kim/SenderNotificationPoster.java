package app.intern.kim;

import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.graphics.Bitmap;
import android.os.Build;
import androidx.core.app.NotificationCompat;
import androidx.core.app.NotificationManagerCompat;
import androidx.core.app.Person;
import androidx.core.content.pm.ShortcutInfoCompat;
import androidx.core.content.pm.ShortcutManagerCompat;
import androidx.core.graphics.drawable.IconCompat;
import java.util.Map;

final class SenderNotificationPoster {

    private static final String channelID = "internkim";
    private static final String tapKey = "google.message_id";

    private final Context context;

    SenderNotificationPoster(Context context) {
        this.context = context;
    }

    void post(SentNotification sent, Bitmap picture) {
        ensureChannel();
        IconCompat icon = IconCompat.createWithBitmap(picture);
        String senderID = senderIDOf(sent);
        Person sender = new Person.Builder().setName(sent.nameShown()).setIcon(icon).setKey(senderID).build();
        String standingTag = sent.tag.isEmpty() ? sent.messageID : sent.tag;

        ShortcutManagerCompat.pushDynamicShortcut(
            context,
            new ShortcutInfoCompat.Builder(context, senderID)
                .setLongLived(true)
                .setShortLabel(sent.nameShown())
                .setIcon(icon)
                .setPerson(sender)
                .setIntent(new Intent(Intent.ACTION_VIEW, null, context, MainActivity.class))
                .build()
        );

        NotificationCompat.MessagingStyle style = new NotificationCompat.MessagingStyle(sender).addMessage(
            sent.textShown(),
            System.currentTimeMillis(),
            sender
        );

        NotificationCompat.Builder builder = new NotificationCompat.Builder(context, channelID)
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentTitle(sent.nameShown())
            .setContentText(sent.textShown())
            .setStyle(style)
            .setShortcutId(senderID)
            .setCategory(NotificationCompat.CATEGORY_MESSAGE)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setAutoCancel(true)
            .setContentIntent(tapIntent(sent, standingTag));

        if (NotificationManagerCompat.from(context).areNotificationsEnabled()) {
            NotificationManagerCompat.from(context).notify(standingTag, 0, builder.build());
        }
    }

    private static String senderIDOf(SentNotification sent) {
        return "sender:" + sent.nameShown();
    }

    private PendingIntent tapIntent(SentNotification sent, String standingTag) {
        Intent opening = new Intent(context, MainActivity.class).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_CLEAR_TOP);
        opening.putExtra(tapKey, sent.messageID);
        for (Map.Entry<String, String> entry : sent.data.entrySet()) {
            opening.putExtra(entry.getKey(), entry.getValue());
        }
        return PendingIntent.getActivity(
            context,
            standingTag.hashCode(),
            opening,
            PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE
        );
    }

    private void ensureChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return;
        NotificationManager manager = context.getSystemService(NotificationManager.class);
        if (manager.getNotificationChannel(channelID) != null) return;
        manager.createNotificationChannel(
            new NotificationChannel(channelID, context.getString(R.string.notification_channel_name), NotificationManager.IMPORTANCE_HIGH)
        );
    }
}

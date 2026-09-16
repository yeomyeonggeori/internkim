package app.intern.kim;

import androidx.annotation.NonNull;
import androidx.annotation.Nullable;
import com.google.firebase.messaging.RemoteMessage;
import java.util.Collections;
import java.util.HashMap;
import java.util.Map;

final class SentNotification {

    final String messageID;
    final String title;
    final String body;
    final String tag;
    final String senderID;
    final String senderName;
    final String pictureURL;
    final Map<String, String> data;

    private SentNotification(String messageID, Map<String, String> data) {
        this.messageID = messageID;
        this.data = Collections.unmodifiableMap(new HashMap<>(data));
        this.title = valueOf(data, "title");
        this.body = valueOf(data, "body");
        this.tag = valueOf(data, "tag");
        this.senderID = valueOf(data, "senderID");
        this.senderName = valueOf(data, "senderName");
        this.pictureURL = valueOf(data, "pictureURL");
    }

    @Nullable
    static SentNotification from(@NonNull RemoteMessage remoteMessage) {
        if (remoteMessage.getNotification() != null) return null;
        Map<String, String> data = remoteMessage.getData();
        if (valueOf(data, "title").isEmpty()) return null;
        String messageID = remoteMessage.getMessageId();
        return new SentNotification(messageID == null ? valueOf(data, "tag") : messageID, data);
    }

    String nameShown() {
        return senderName.isEmpty() ? title : senderName;
    }

    String senderKey() {
        return senderID.isEmpty() ? "name:" + nameShown() : "member:" + senderID;
    }

    String textShown() {
        if (senderName.isEmpty() || senderName.equals(title)) return body;
        return body.isEmpty() ? title : title + "\n" + body;
    }

    private static String valueOf(Map<String, String> data, String key) {
        String value = data.get(key);
        return value == null ? "" : value.trim();
    }
}

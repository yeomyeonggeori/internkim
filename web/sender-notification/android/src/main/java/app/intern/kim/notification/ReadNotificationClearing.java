package app.intern.kim.notification;

import android.content.Context;
import androidx.core.app.NotificationManagerCompat;
import java.util.Map;

public final class ReadNotificationClearing {

    private static final String readTagKey = "readTag";

    private ReadNotificationClearing() {}

    public static boolean isFor(Map<String, String> data) {
        String tag = data.get(readTagKey);
        return tag != null && !tag.isEmpty();
    }

    public static void clear(Context context, Map<String, String> data) {
        NotificationManagerCompat.from(context).cancel(data.get(readTagKey), 0);
    }
}

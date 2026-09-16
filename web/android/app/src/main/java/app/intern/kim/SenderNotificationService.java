package app.intern.kim;

import androidx.annotation.NonNull;
import com.capacitorjs.plugins.pushnotifications.MessagingService;
import com.google.firebase.messaging.RemoteMessage;

public class SenderNotificationService extends MessagingService {

    @Override
    public void onMessageReceived(@NonNull RemoteMessage remoteMessage) {
        SentNotification sent = SentNotification.from(remoteMessage);
        if (sent != null) {
            new SenderNotificationPoster(this).post(sent, SenderPicture.of(this, sent.pictureURL));
        }
        super.onMessageReceived(remoteMessage);
    }
}

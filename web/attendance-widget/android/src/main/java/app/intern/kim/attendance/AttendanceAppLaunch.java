package app.intern.kim.attendance;

import android.content.Context;
import android.content.Intent;
import java.util.Objects;

final class AttendanceAppLaunch {

    private AttendanceAppLaunch() {}

    static Intent openingIntent(Context context) {
        Intent launch = context.getPackageManager().getLaunchIntentForPackage(context.getPackageName());
        return Objects.requireNonNull(launch, "the app declares no launcher activity for the widget to open")
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
    }
}

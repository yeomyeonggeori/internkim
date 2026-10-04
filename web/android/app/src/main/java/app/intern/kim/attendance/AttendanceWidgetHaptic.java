package app.intern.kim.attendance;

import android.content.Context;
import android.media.AudioAttributes;
import android.os.Build;
import android.os.VibrationEffect;
import android.os.Vibrator;
import android.os.VibratorManager;
import androidx.annotation.Nullable;

final class AttendanceWidgetHaptic {

    private static final long[] successPattern = { 0, 30 };
    private static final long[] failurePattern = { 0, 40, 80, 40 };
    private static final AudioAttributes notificationUsage = new AudioAttributes.Builder()
        .setUsage(AudioAttributes.USAGE_NOTIFICATION)
        .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
        .build();

    private AttendanceWidgetHaptic() {}

    static void success(Context context) {
        play(context, successPattern);
    }

    static void failure(Context context) {
        play(context, failurePattern);
    }

    private static void play(Context context, long[] pattern) {
        Vibrator vibrator = vibratorOf(context);
        if (vibrator == null || !vibrator.hasVibrator()) return;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            vibrator.vibrate(VibrationEffect.createWaveform(pattern, -1), notificationUsage);
        } else {
            vibrator.vibrate(pattern, -1, notificationUsage);
        }
    }

    @Nullable
    private static Vibrator vibratorOf(Context context) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            VibratorManager manager = context.getSystemService(VibratorManager.class);
            return manager == null ? null : manager.getDefaultVibrator();
        }
        return context.getSystemService(Vibrator.class);
    }
}

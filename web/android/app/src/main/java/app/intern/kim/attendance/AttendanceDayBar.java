package app.intern.kim.attendance;

import android.content.Context;
import android.graphics.Bitmap;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Paint;
import android.graphics.Path;
import android.graphics.RectF;
import androidx.annotation.Nullable;
import app.intern.kim.R;
import java.util.List;

final class AttendanceDayBar {

    private static final int heightDP = 6;
    private static final int fallbackWidthDP = 120;

    private AttendanceDayBar() {}

    static Bitmap draw(Context context, List<AttendanceToday.Bar> bars, List<WorkLocation> locations, int widthDP) {
        float density = context.getResources().getDisplayMetrics().density;
        int width = Math.round((widthDP > 0 ? widthDP : fallbackWidthDP) * density);
        int height = Math.round(heightDP * density);
        Bitmap bitmap = Bitmap.createBitmap(width, height, Bitmap.Config.ARGB_8888);
        Canvas canvas = new Canvas(bitmap);
        RectF track = new RectF(0, 0, width, height);
        float radius = height / 2f;

        Paint paint = new Paint(Paint.ANTI_ALIAS_FLAG);
        paint.setColor(context.getColor(R.color.attendance_widget_track));
        canvas.drawRoundRect(track, radius, radius, paint);

        Path rounded = new Path();
        rounded.addRoundRect(track, radius, radius, Path.Direction.CW);
        canvas.clipPath(rounded);

        float start = 0;
        for (AttendanceToday.Bar bar : bars) {
            float barWidth = (float) (width * Math.min(1, bar.widthPercent / 100));
            paint.setColor(colorOf(context, colorNamed(bar.location, locations)));
            canvas.drawRect(start, 0, start + barWidth, height, paint);
            start += barWidth;
        }
        return bitmap;
    }

    static int colorOf(Context context, @Nullable String hex) {
        int fallback = context.getColor(R.color.attendance_widget_text_secondary);
        if (hex == null) return fallback;
        String digits = hex.startsWith("#") ? hex.substring(1) : hex;
        if (digits.length() != 6) return fallback;
        try {
            return Color.parseColor("#" + digits);
        } catch (IllegalArgumentException notAColor) {
            return fallback;
        }
    }

    @Nullable
    private static String colorNamed(@Nullable String name, List<WorkLocation> locations) {
        if (name == null) return null;
        for (WorkLocation location : locations) {
            if (location.name.equals(name)) return location.color;
        }
        return null;
    }
}

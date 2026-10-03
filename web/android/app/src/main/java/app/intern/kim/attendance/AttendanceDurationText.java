package app.intern.kim.attendance;

import android.content.Context;
import android.graphics.Color;
import android.text.SpannableStringBuilder;
import android.text.Spanned;
import android.text.style.ForegroundColorSpan;
import android.text.style.RelativeSizeSpan;
import app.intern.kim.R;
import java.util.Locale;

final class AttendanceDurationText {

    private static final float unitSize = 0.8f;
    private static final float unitAlpha = 0.6f;
    private static final float leadingZeroAlpha = 0.4f;

    private AttendanceDurationText() {}

    static CharSequence of(Context context, int minutes, int color) {
        SpannableStringBuilder text = new SpannableStringBuilder();
        appendPart(text, twoDigits(minutes / 60), context.getString(R.string.attendance_widget_hours), color);
        text.append(' ');
        appendPart(text, twoDigits(minutes % 60), context.getString(R.string.attendance_widget_minutes), color);
        return text;
    }

    private static String twoDigits(int value) {
        return String.format(Locale.US, "%02d", value);
    }

    private static boolean dimsLeadingZero(String digits) {
        return digits.startsWith("0") && !digits.equals("00");
    }

    private static void appendPart(SpannableStringBuilder text, String digits, String unit, int color) {
        int digitsStart = text.length();
        text.append(digits);
        if (dimsLeadingZero(digits)) {
            text.setSpan(new ForegroundColorSpan(faded(color, leadingZeroAlpha)), digitsStart, digitsStart + 1, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        }
        int unitStart = text.length();
        text.append(unit);
        text.setSpan(new RelativeSizeSpan(unitSize), unitStart, text.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        text.setSpan(new ForegroundColorSpan(faded(color, unitAlpha)), unitStart, text.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
    }

    private static int faded(int color, float alpha) {
        return Color.argb(Math.round(Color.alpha(color) * alpha), Color.red(color), Color.green(color), Color.blue(color));
    }
}

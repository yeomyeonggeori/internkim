package app.intern.kim.attendance;

import android.appwidget.AppWidgetManager;
import android.os.Build;
import android.os.Bundle;
import android.util.SizeF;
import android.util.TypedValue;
import android.widget.RemoteViews;
import androidx.annotation.RequiresApi;
import app.intern.kim.R;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

final class AttendanceWidgetSize {

    private static final int mediumMinimumWidthDP = 250;
    private static final int tallMinimumHeightDP = 180;
    private static final int horizontalPaddingDP = 32;
    private static final int tallBarHeightDP = 8;
    private static final int shortBarHeightDP = 6;
    private static final int tallButtonHeightDP = 56;
    private static final float tallCaptionSizeSP = 16f;
    private static final float tallButtonLabelSizeSP = 18f;

    private static final int[] tallButtons = {
        R.id.attendance_widget_primary,
        R.id.attendance_widget_other,
        R.id.attendance_widget_pick_0,
        R.id.attendance_widget_pick_1,
        R.id.attendance_widget_pick_2,
        R.id.attendance_widget_pick_3,
        R.id.attendance_widget_pick_4,
        R.id.attendance_widget_pick_5,
    };
    private static final int[] tallLabels = {
        R.id.attendance_widget_primary_label,
        R.id.attendance_widget_other_label,
        R.id.attendance_widget_pick_0,
        R.id.attendance_widget_pick_1,
        R.id.attendance_widget_pick_2,
        R.id.attendance_widget_pick_3,
        R.id.attendance_widget_pick_4,
        R.id.attendance_widget_pick_5,
    };

    final float widthDP;
    final float heightDP;

    AttendanceWidgetSize(float widthDP, float heightDP) {
        this.widthDP = widthDP;
        this.heightDP = heightDP;
    }

    static List<AttendanceWidgetSize> of(Bundle options) {
        List<AttendanceWidgetSize> sizes = new ArrayList<>();
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            ArrayList<SizeF> offered = offeredSizes(options);
            if (offered != null) {
                for (SizeF size : offered) sizes.add(new AttendanceWidgetSize(size.getWidth(), size.getHeight()));
            }
        }
        if (sizes.isEmpty()) {
            sizes.add(
                new AttendanceWidgetSize(
                    options.getInt(AppWidgetManager.OPTION_APPWIDGET_MIN_WIDTH, 0),
                    options.getInt(AppWidgetManager.OPTION_APPWIDGET_MAX_HEIGHT, 0)
                )
            );
        }
        return Collections.unmodifiableList(sizes);
    }

    @RequiresApi(Build.VERSION_CODES.S)
    @SuppressWarnings("deprecation")
    private static ArrayList<SizeF> offeredSizes(Bundle options) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            return options.getParcelableArrayList(AppWidgetManager.OPTION_APPWIDGET_SIZES, SizeF.class);
        }
        return options.getParcelableArrayList(AppWidgetManager.OPTION_APPWIDGET_SIZES);
    }

    boolean isMedium() {
        return widthDP >= mediumMinimumWidthDP;
    }

    boolean isTall() {
        return heightDP >= tallMinimumHeightDP;
    }

    int barWidthDP() {
        return Math.max(0, Math.round(widthDP) - horizontalPaddingDP);
    }

    int barHeightDP() {
        return isTall() ? tallBarHeightDP : shortBarHeightDP;
    }

    float timeSizeSP() {
        if (isTall()) return isMedium() ? 38f : 26f;
        return isMedium() ? 28f : 25f;
    }

    void scale(RemoteViews views) {
        views.setTextViewTextSize(R.id.attendance_widget_worked, TypedValue.COMPLEX_UNIT_SP, timeSizeSP());
        if (!isTall() || Build.VERSION.SDK_INT < Build.VERSION_CODES.S) return;
        views.setTextViewTextSize(R.id.attendance_widget_whereabouts, TypedValue.COMPLEX_UNIT_SP, tallCaptionSizeSP);
        for (int mark : new int[] { R.id.attendance_widget_mark_in, R.id.attendance_widget_mark_out, R.id.attendance_widget_mark_location }) {
            views.setTextViewTextSize(mark, TypedValue.COMPLEX_UNIT_SP, tallCaptionSizeSP);
        }
        views.setViewLayoutHeight(R.id.attendance_widget_day_bar, barHeightDP(), TypedValue.COMPLEX_UNIT_DIP);
        for (int button : tallButtons) views.setViewLayoutHeight(button, tallButtonHeightDP, TypedValue.COMPLEX_UNIT_DIP);
        for (int label : tallLabels) views.setTextViewTextSize(label, TypedValue.COMPLEX_UNIT_SP, tallButtonLabelSizeSP);
    }

    SizeF asSize() {
        return new SizeF(widthDP, heightDP);
    }
}

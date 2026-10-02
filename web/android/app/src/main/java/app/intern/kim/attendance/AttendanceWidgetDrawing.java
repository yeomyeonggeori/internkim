package app.intern.kim.attendance;

import android.app.PendingIntent;
import android.appwidget.AppWidgetManager;
import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import android.os.Bundle;
import android.os.SystemClock;
import android.text.SpannableString;
import android.text.Spanned;
import android.text.style.ForegroundColorSpan;
import android.view.View;
import android.widget.RemoteViews;
import androidx.annotation.Nullable;
import app.intern.kim.MainActivity;
import app.intern.kim.R;
import java.util.List;

final class AttendanceWidgetDrawing {

    private static final int mediumMinimumWidthDP = 250;
    private static final int horizontalPaddingDP = 28;
    private static final int pickerColumnsAtMost = 3;
    private static final int[] pickerButtons = {
        R.id.attendance_widget_pick_0,
        R.id.attendance_widget_pick_1,
        R.id.attendance_widget_pick_2,
        R.id.attendance_widget_pick_3,
        R.id.attendance_widget_pick_4,
        R.id.attendance_widget_pick_5,
    };

    private AttendanceWidgetDrawing() {}

    static RemoteViews views(
        Context context,
        AttendanceWidgetEntry entry,
        int widgetID,
        Bundle options,
        @Nullable String refusal,
        boolean isChoosingLocation
    ) {
        RemoteViews views = new RemoteViews(context.getPackageName(), R.layout.attendance_widget);
        views.setOnClickPendingIntent(R.id.attendance_widget_root, openApp(context));
        int widthDP = options.getInt(AppWidgetManager.OPTION_APPWIDGET_MIN_WIDTH, 0);
        boolean isMedium = widthDP >= mediumMinimumWidthDP;

        if (entry.failure != null) {
            views.setViewVisibility(R.id.attendance_widget_failure, View.VISIBLE);
            views.setViewVisibility(R.id.attendance_widget_today, View.GONE);
            views.setViewVisibility(R.id.attendance_widget_picker, View.GONE);
            views.setTextViewText(R.id.attendance_widget_failure, entry.failure);
            return views;
        }
        views.setViewVisibility(R.id.attendance_widget_failure, View.GONE);

        AttendanceDefaultWorkplace choice = AttendanceDefaultWorkplace.of(
            AttendanceWidgetStore.workplace(context, widgetID),
            entry.locations
        );
        List<WorkLocation> others = AttendanceDefaultWorkplace.others(choice.location, entry.locations);

        if (isMedium && isChoosingLocation && !others.isEmpty()) {
            views.setViewVisibility(R.id.attendance_widget_today, View.GONE);
            views.setViewVisibility(R.id.attendance_widget_picker, View.VISIBLE);
            drawPicker(context, views, others);
            return views;
        }
        views.setViewVisibility(R.id.attendance_widget_today, View.VISIBLE);
        views.setViewVisibility(R.id.attendance_widget_picker, View.GONE);

        drawWorkedTime(context, views, entry);
        drawWhereabouts(context, views, entry, choice, isMedium);
        views.setImageViewBitmap(
            R.id.attendance_widget_day_bar,
            AttendanceDayBar.draw(context, entry.today.bars(entry.currentTime()), entry.locations, Math.max(0, widthDP - horizontalPaddingDP))
        );
        drawStatus(context, views, entry, refusal, isMedium);
        drawButtons(context, views, entry, choice, others, isMedium);
        return views;
    }

    private static void drawWorkedTime(Context context, RemoteViews views, AttendanceWidgetEntry entry) {
        long elapsed = entry.today.elapsedMillis(entry.date);
        long base = SystemClock.elapsedRealtime() - elapsed;
        boolean isWorking = entry.today.isWorking();
        int color = !isWorking && elapsed == 0 ? R.color.attendance_widget_text_dimmed : R.color.attendance_widget_text;
        views.setChronometer(R.id.attendance_widget_worked, base, null, isWorking);
        views.setTextColor(R.id.attendance_widget_worked, context.getColor(color));
    }

    private static void drawWhereabouts(
        Context context,
        RemoteViews views,
        AttendanceWidgetEntry entry,
        AttendanceDefaultWorkplace choice,
        boolean isMedium
    ) {
        if (!isMedium) {
            views.setViewVisibility(R.id.attendance_widget_whereabouts, View.GONE);
            return;
        }
        views.setViewVisibility(R.id.attendance_widget_whereabouts, View.VISIBLE);
        String shown;
        if (entry.today.isWorking() && entry.today.location() != null) {
            shown = entry.today.location();
        } else if (entry.today.isWorking()) {
            shown = context.getString(R.string.attendance_widget_working);
        } else if (choice.location != null) {
            shown = context.getString(R.string.attendance_widget_default_location, choice.location.name);
        } else if (choice.goneName != null) {
            shown = context.getString(R.string.attendance_widget_location_gone, choice.goneName);
        } else {
            shown = context.getString(R.string.attendance_widget_no_workplace);
        }
        views.setTextViewText(R.id.attendance_widget_whereabouts, shown);
    }

    private static void drawStatus(
        Context context,
        RemoteViews views,
        AttendanceWidgetEntry entry,
        @Nullable String refusal,
        boolean isMedium
    ) {
        if (refusal != null) {
            views.setViewVisibility(R.id.attendance_widget_marks, View.GONE);
            views.setViewVisibility(R.id.attendance_widget_refusal, View.VISIBLE);
            views.setTextViewText(R.id.attendance_widget_refusal, refusal);
            return;
        }
        views.setViewVisibility(R.id.attendance_widget_refusal, View.GONE);
        views.setViewVisibility(R.id.attendance_widget_marks, View.VISIBLE);

        StringBuilder marks = new StringBuilder();
        AttendanceToday today = entry.today;
        if (today.clockInTime() != null) {
            marks.append(context.getString(R.string.attendance_widget_mark_clock_in, today.clockInTime()));
        }
        if (today.clockOutTime() != null && !today.isWorking()) {
            appendMark(marks, context.getString(R.string.attendance_widget_mark_clock_out, today.clockOutTime()));
        }
        if (!isMedium && today.isWorking() && today.location() != null) {
            appendMark(marks, context.getString(R.string.attendance_widget_mark_location, today.location()));
        }
        views.setTextViewText(R.id.attendance_widget_marks, marks);
    }

    private static void appendMark(StringBuilder marks, String mark) {
        if (marks.length() > 0) marks.append("   ");
        marks.append(mark);
    }

    private static void drawButtons(
        Context context,
        RemoteViews views,
        AttendanceWidgetEntry entry,
        AttendanceDefaultWorkplace choice,
        List<WorkLocation> others,
        boolean isMedium
    ) {
        boolean offersOthers = isMedium && !others.isEmpty();
        views.setViewVisibility(R.id.attendance_widget_notice, View.GONE);
        views.setViewVisibility(R.id.attendance_widget_primary, View.VISIBLE);
        views.setViewVisibility(R.id.attendance_widget_other, offersOthers ? View.VISIBLE : View.GONE);
        views.setOnClickPendingIntent(R.id.attendance_widget_other, action(context, AttendanceWidgetProvider.actionShowLocations, null));

        if (entry.today.isWorking()) {
            views.setViewVisibility(R.id.attendance_widget_other, View.GONE);
            views.setTextViewText(R.id.attendance_widget_primary, context.getString(R.string.attendance_widget_clock_out));
            views.setInt(R.id.attendance_widget_primary, "setBackgroundResource", R.drawable.attendance_widget_button);
            views.setTextColor(R.id.attendance_widget_primary, context.getColor(R.color.attendance_widget_text));
            views.setOnClickPendingIntent(R.id.attendance_widget_primary, action(context, AttendanceWidgetProvider.actionClockOut, null));
            return;
        }

        if (choice.needsChoice()) {
            views.setViewVisibility(R.id.attendance_widget_primary, View.GONE);
            if (!offersOthers) views.setViewVisibility(R.id.attendance_widget_notice, View.VISIBLE);
            return;
        }

        String label = isMedium && choice.location != null
            ? context.getString(R.string.attendance_widget_clock_in_at, choice.location.name)
            : context.getString(R.string.attendance_widget_clock_in);
        String location = choice.location == null ? null : choice.location.name;
        views.setTextViewText(R.id.attendance_widget_primary, label);
        views.setInt(R.id.attendance_widget_primary, "setBackgroundResource", R.drawable.attendance_widget_button_prominent);
        views.setTextColor(R.id.attendance_widget_primary, context.getColor(R.color.attendance_widget_on_accent));
        views.setOnClickPendingIntent(R.id.attendance_widget_primary, action(context, AttendanceWidgetProvider.actionClockIn, location));
    }

    private static void drawPicker(Context context, RemoteViews views, List<WorkLocation> locations) {
        views.setOnClickPendingIntent(R.id.attendance_widget_back, action(context, AttendanceWidgetProvider.actionHideLocations, null));
        int offered = Math.min(locations.size(), pickerButtons.length);
        int perRow = pickerColumns(offered);
        for (int slot = 0; slot < pickerButtons.length; slot++) {
            int button = pickerButtons[slot];
            int column = slot % pickerColumnsAtMost;
            int index = (slot / pickerColumnsAtMost) * perRow + column;
            if (column >= perRow) {
                views.setViewVisibility(button, View.GONE);
            } else if (index >= offered) {
                views.setViewVisibility(button, View.INVISIBLE);
            } else {
                WorkLocation location = locations.get(index);
                views.setViewVisibility(button, View.VISIBLE);
                views.setTextViewText(button, dotted(context, location));
                views.setOnClickPendingIntent(button, action(context, AttendanceWidgetProvider.actionClockIn, location.name));
            }
        }
        views.setViewVisibility(R.id.attendance_widget_pick_row_1, offered > perRow ? View.VISIBLE : View.GONE);
    }

    private static int pickerColumns(int offered) {
        if (offered <= 2) return Math.max(1, offered);
        return offered <= 4 ? 2 : pickerColumnsAtMost;
    }

    private static CharSequence dotted(Context context, WorkLocation location) {
        SpannableString named = new SpannableString("● " + location.name);
        named.setSpan(new ForegroundColorSpan(AttendanceDayBar.colorOf(context, location.color)), 0, 1, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        return named;
    }

    private static PendingIntent action(Context context, String action, @Nullable String location) {
        Intent intent = new Intent(context, AttendanceWidgetProvider.class).setAction(action);
        if (location != null) {
            intent.putExtra(AttendanceWidgetProvider.locationExtra, location);
            intent.setData(Uri.fromParts("attendance", location, null));
        }
        return PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
    }

    private static PendingIntent openApp(Context context) {
        Intent intent = new Intent(context, MainActivity.class).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        return PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
    }
}

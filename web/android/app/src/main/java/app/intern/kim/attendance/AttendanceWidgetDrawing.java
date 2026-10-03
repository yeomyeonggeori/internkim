package app.intern.kim.attendance;

import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import android.text.SpannableString;
import android.text.Spanned;
import android.text.style.ForegroundColorSpan;
import android.text.style.RelativeSizeSpan;
import android.view.View;
import android.widget.RemoteViews;
import androidx.annotation.Nullable;
import app.intern.kim.MainActivity;
import app.intern.kim.R;
import java.util.List;

final class AttendanceWidgetDrawing {

    private static final int pickerColumnsAtMost = 3;
    private static final float locationDotSize = 0.7f;
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
        AttendanceWidgetSize size,
        @Nullable String refusal,
        boolean isChoosingLocation
    ) {
        RemoteViews views = new RemoteViews(context.getPackageName(), R.layout.attendance_widget);
        views.setOnClickPendingIntent(R.id.attendance_widget_root, openApp(context));
        boolean isMedium = size.isMedium();

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
            drawPicker(context, views, others, size.isTall());
            return views;
        }
        views.setViewVisibility(R.id.attendance_widget_today, View.VISIBLE);
        views.setViewVisibility(R.id.attendance_widget_picker, View.GONE);

        size.scale(views);
        drawWorkedTime(context, views, entry);
        drawWhereabouts(context, views, entry, choice, isMedium);
        views.setImageViewBitmap(
            R.id.attendance_widget_day_bar,
            AttendanceDayBar.draw(context, entry.today.bars(entry.currentTime()), entry.locations, size.barWidthDP(), size.barHeightDP())
        );
        drawStatus(views, entry, refusal, isMedium);
        drawButtons(context, views, entry, choice, others, isMedium);
        return views;
    }

    private static void drawWorkedTime(Context context, RemoteViews views, AttendanceWidgetEntry entry) {
        int minutes = entry.today.elapsedMinutes(entry.date);
        boolean isDimmed = !entry.today.isWorking() && minutes == 0;
        int color = context.getColor(isDimmed ? R.color.attendance_widget_text_dimmed : R.color.attendance_widget_text);
        views.setTextViewText(R.id.attendance_widget_worked, AttendanceDurationText.of(context, minutes, color));
        views.setTextColor(R.id.attendance_widget_worked, color);
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
        RemoteViews views,
        AttendanceWidgetEntry entry,
        @Nullable String refusal,
        boolean isMedium
    ) {
        if (refusal != null) {
            views.setViewVisibility(R.id.attendance_widget_marks, View.GONE);
            views.setViewVisibility(R.id.attendance_widget_refusal_row, View.VISIBLE);
            views.setTextViewText(R.id.attendance_widget_refusal, refusal);
            return;
        }
        views.setViewVisibility(R.id.attendance_widget_refusal_row, View.GONE);
        views.setViewVisibility(R.id.attendance_widget_marks, View.VISIBLE);

        AttendanceToday today = entry.today;
        drawMark(views, R.id.attendance_widget_mark_in_icon, R.id.attendance_widget_mark_in, today.clockInTime());
        drawMark(views, R.id.attendance_widget_mark_out_icon, R.id.attendance_widget_mark_out, today.isWorking() ? null : today.clockOutTime());
        drawMark(
            views,
            R.id.attendance_widget_mark_location_icon,
            R.id.attendance_widget_mark_location,
            !isMedium && today.isWorking() ? today.location() : null
        );
    }

    private static void drawMark(RemoteViews views, int icon, int label, @Nullable String shown) {
        int visibility = shown == null ? View.GONE : View.VISIBLE;
        views.setViewVisibility(icon, visibility);
        views.setViewVisibility(label, visibility);
        if (shown != null) views.setTextViewText(label, shown);
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
            drawPrimary(context, views, context.getString(R.string.attendance_widget_clock_out), false);
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
        drawPrimary(context, views, label, true);
        views.setOnClickPendingIntent(R.id.attendance_widget_primary, action(context, AttendanceWidgetProvider.actionClockIn, location));
    }

    private static void drawPrimary(Context context, RemoteViews views, String label, boolean isProminent) {
        views.setTextViewText(R.id.attendance_widget_primary_label, label);
        views.setViewVisibility(R.id.attendance_widget_primary_icon, isProminent ? View.VISIBLE : View.GONE);
        views.setInt(
            R.id.attendance_widget_primary,
            "setBackgroundResource",
            isProminent ? R.drawable.attendance_widget_button_prominent : R.drawable.attendance_widget_button
        );
        views.setTextColor(
            R.id.attendance_widget_primary_label,
            context.getColor(isProminent ? R.color.attendance_widget_on_accent : R.color.attendance_widget_accent)
        );
    }

    private static void drawPicker(Context context, RemoteViews views, List<WorkLocation> locations, boolean isTall) {
        views.setOnClickPendingIntent(R.id.attendance_widget_back, action(context, AttendanceWidgetProvider.actionHideLocations, null));
        int offered = Math.min(locations.size(), isTall ? pickerButtons.length : pickerColumnsAtMost);
        int perRow = isTall ? pickerColumns(offered) : Math.max(1, offered);
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
        named.setSpan(new RelativeSizeSpan(locationDotSize), 0, 1, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
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

package app.intern.kim.attendance;

import android.app.Activity;
import android.appwidget.AppWidgetManager;
import android.content.Intent;
import android.os.Bundle;
import androidx.appcompat.app.AlertDialog;
import androidx.appcompat.app.AppCompatActivity;
import app.intern.kim.R;
import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public class AttendanceWidgetConfigureActivity extends AppCompatActivity {

    private static final ExecutorService loader = Executors.newSingleThreadExecutor();

    private int widgetID = AppWidgetManager.INVALID_APPWIDGET_ID;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        Bundle extras = getIntent().getExtras();
        if (extras != null) widgetID = extras.getInt(AppWidgetManager.EXTRA_APPWIDGET_ID, widgetID);
        if (widgetID == AppWidgetManager.INVALID_APPWIDGET_ID) {
            finish();
            return;
        }
        setResult(Activity.RESULT_CANCELED, resultIntent());
        loader.execute(this::loadLocations);
    }

    private void loadLocations() {
        try {
            List<WorkLocation> locations = AttendanceAPI.held(this).settings().workLocations;
            runOnUiThread(() -> offer(locations));
        } catch (AttendanceAPI.Failure failure) {
            runOnUiThread(() -> explain(failure.getMessage()));
        }
    }

    private void offer(List<WorkLocation> locations) {
        if (isFinishing()) return;
        if (locations.isEmpty()) {
            explain(getString(R.string.attendance_widget_no_workplace));
            return;
        }
        String held = AttendanceWidgetStore.workplace(this, widgetID);
        String[] names = new String[locations.size()];
        int checked = 0;
        for (int index = 0; index < locations.size(); index++) {
            names[index] = locations.get(index).name;
            if (names[index].equals(held)) checked = index;
        }
        new AlertDialog.Builder(this)
            .setTitle(R.string.attendance_widget_default_workplace)
            .setSingleChoiceItems(names, checked, (dialog, chosen) -> {
                AttendanceWidgetStore.keepWorkplace(this, widgetID, names[chosen]);
                dialog.dismiss();
                keep();
            })
            .setOnCancelListener(dialog -> finish())
            .show();
    }

    private void explain(String message) {
        if (isFinishing()) return;
        new AlertDialog.Builder(this)
            .setTitle(R.string.attendance_widget_name)
            .setMessage(message)
            .setPositiveButton(android.R.string.ok, (dialog, which) -> keep())
            .setOnCancelListener(dialog -> keep())
            .show();
    }

    private void keep() {
        setResult(Activity.RESULT_OK, resultIntent());
        AttendanceWidgetProvider.redrawSoon(this);
        finish();
    }

    private Intent resultIntent() {
        return new Intent().putExtra(AppWidgetManager.EXTRA_APPWIDGET_ID, widgetID);
    }
}

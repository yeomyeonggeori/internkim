package app.intern.kim.attendance;

import androidx.annotation.Nullable;
import java.util.ArrayList;
import java.util.List;
import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

final class AttendanceJSON {

    private AttendanceJSON() {}

    static AttendanceRow row(JSONObject object) throws JSONException {
        return new AttendanceRow(
            object.getString("eventID"),
            object.getString("kind"),
            object.getString("date"),
            object.getString("time"),
            object.getString("occurredAt"),
            optionalString(object, "location")
        );
    }

    static JSONObject of(AttendanceRow row) throws JSONException {
        return new JSONObject()
            .put("eventID", row.eventID)
            .put("kind", row.kind)
            .put("date", row.date)
            .put("time", row.time)
            .put("occurredAt", row.occurredAt)
            .put("location", row.location == null ? JSONObject.NULL : row.location);
    }

    static List<AttendanceRow> rows(JSONArray array) throws JSONException {
        List<AttendanceRow> rows = new ArrayList<>();
        for (int index = 0; index < array.length(); index++) rows.add(row(array.getJSONObject(index)));
        return rows;
    }

    static JSONArray ofRows(List<AttendanceRow> rows) throws JSONException {
        JSONArray array = new JSONArray();
        for (AttendanceRow row : rows) array.put(of(row));
        return array;
    }

    static CompanySettings settings(JSONObject object) throws JSONException {
        JSONArray listed = object.getJSONArray("workLocations");
        List<WorkLocation> locations = new ArrayList<>();
        for (int index = 0; index < listed.length(); index++) {
            JSONObject location = listed.getJSONObject(index);
            locations.add(new WorkLocation(location.getString("name"), optionalString(location, "color")));
        }
        return new CompanySettings(object.getString("timeZone"), locations);
    }

    static JSONObject of(CompanySettings settings) throws JSONException {
        JSONArray locations = new JSONArray();
        for (WorkLocation location : settings.workLocations) {
            locations.put(
                new JSONObject()
                    .put("name", location.name)
                    .put("color", location.color == null ? JSONObject.NULL : location.color)
            );
        }
        return new JSONObject().put("timeZone", settings.timeZone).put("workLocations", locations);
    }

    static AttendanceWrite write(JSONObject object) throws JSONException {
        JSONObject event = object.optJSONObject("event");
        return new AttendanceWrite(
            object.getString("status"),
            optionalString(object, "eventID"),
            event == null
                ? null
                : new AttendanceWrite.Event(
                    event.getString("id"),
                    event.getString("kind"),
                    event.getString("occurredAt"),
                    optionalString(event, "location")
                )
        );
    }

    static AttendanceWidgetCache cache(JSONObject object) throws JSONException {
        AttendanceWidgetCache cache = new AttendanceWidgetCache(object.getString("origin"));
        JSONObject settings = object.optJSONObject("settings");
        cache.settings = settings == null ? null : settings(settings);
        cache.settingsFetchedAt = optionalMoment(object, "settingsFetchedAt");
        cache.rows = rows(object.getJSONArray("rows"));
        cache.rowsTrustedUntil = optionalMoment(object, "rowsTrustedUntil");
        JSONObject pending = object.optJSONObject("pending");
        cache.pending = pending == null ? null : row(pending);
        cache.rowsListedAt = optionalMoment(object, "rowsListedAt");
        cache.tappedAt = optionalMoment(object, "tappedAt");
        return cache;
    }

    static JSONObject of(AttendanceWidgetCache cache) throws JSONException {
        JSONObject object = new JSONObject().put("origin", cache.origin).put("rows", ofRows(cache.rows));
        if (cache.settings != null) object.put("settings", of(cache.settings));
        if (cache.pending != null) object.put("pending", of(cache.pending));
        object.putOpt("settingsFetchedAt", cache.settingsFetchedAt);
        object.putOpt("rowsTrustedUntil", cache.rowsTrustedUntil);
        object.putOpt("rowsListedAt", cache.rowsListedAt);
        object.putOpt("tappedAt", cache.tappedAt);
        return object;
    }

    @Nullable
    private static String optionalString(JSONObject object, String key) {
        if (!object.has(key) || object.isNull(key)) return null;
        return object.optString(key);
    }

    @Nullable
    private static Long optionalMoment(JSONObject object, String key) {
        if (!object.has(key) || object.isNull(key)) return null;
        return object.optLong(key);
    }
}

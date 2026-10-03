package app.intern.kim.attendance;

import android.content.Context;
import android.util.Log;
import androidx.annotation.Nullable;
import app.intern.kim.R;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.TimeZone;
import org.json.JSONException;
import org.json.JSONObject;

final class AttendanceAPI {

    private static final String logTag = "AttendanceWidget";
    private static final int timeoutMillis = 8000;
    private static final long dayMillis = 24 * 60 * 60 * 1000L;

    final AttendanceWidgetStore.Credential credential;
    private final Context context;

    private AttendanceAPI(Context context, AttendanceWidgetStore.Credential credential) {
        this.context = context;
        this.credential = credential;
    }

    static AttendanceAPI held(Context context) throws Failure {
        AttendanceWidgetStore.Credential credential = AttendanceWidgetStore.credential(context);
        if (credential == null) throw new Failure(context.getString(R.string.attendance_widget_no_key));
        return new AttendanceAPI(context, credential);
    }

    List<AttendanceRow> sinceYesterday(long now, TimeZone timeZone) throws Failure {
        JSONObject input = new JSONObject();
        try {
            input.put("from", CompanyClock.day(now - dayMillis, timeZone)).put("to", CompanyClock.day(now, timeZone));
            return AttendanceJSON.rows(invoke("attendance_list", input).getJSONArray("attendance"));
        } catch (JSONException unreadable) {
            throw unreadableAnswer("attendance_list", unreadable);
        }
    }

    CompanySettings settings() throws Failure {
        try {
            return AttendanceJSON.settings(invoke("company_settings_get", new JSONObject()));
        } catch (JSONException unreadable) {
            throw unreadableAnswer("company_settings_get", unreadable);
        }
    }

    AttendanceWrite clock(String kind, @Nullable String location) throws Failure {
        JSONObject input = new JSONObject();
        try {
            input.put("kind", kind);
            if (location != null && !location.isEmpty() && AttendanceRow.clockIn.equals(kind)) input.put("location", location);
            return AttendanceJSON.write(invoke("attendance_add", input));
        } catch (JSONException unreadable) {
            throw unreadableAnswer("attendance_add", unreadable);
        }
    }

    private JSONObject invoke(String tool, JSONObject input) throws Failure, JSONException {
        HttpURLConnection connection = null;
        try {
            connection = (HttpURLConnection) new URL(credential.origin + "/api/v1/tools/" + tool + "/invoke").openConnection();
            connection.setRequestMethod("POST");
            connection.setConnectTimeout(timeoutMillis);
            connection.setReadTimeout(timeoutMillis);
            connection.setDoOutput(true);
            connection.setRequestProperty("Authorization", "Bearer " + credential.token);
            connection.setRequestProperty("Content-Type", "application/json");
            connection.setRequestProperty("Connection", "close");
            try (OutputStream body = connection.getOutputStream()) {
                body.write(new JSONObject().put("input", input).toString().getBytes(StandardCharsets.UTF_8));
            }
            int status = connection.getResponseCode();
            boolean isAnswered = status >= 200 && status < 300;
            String answer = readAll(isAnswered ? connection.getInputStream() : connection.getErrorStream());
            if (!isAnswered) {
                Log.w(logTag, tool + " answered " + status + ": " + answer);
                throw new Failure(refusalIn(answer, status));
            }
            return new JSONObject(answer).getJSONObject("result");
        } catch (IOException unreachable) {
            Log.w(logTag, "the widget could not reach " + tool + " at " + credential.origin, unreachable);
            throw new Failure(context.getString(R.string.attendance_widget_unreachable), unreachable);
        } finally {
            if (connection != null) connection.disconnect();
        }
    }

    private String refusalIn(String answer, int status) {
        String unexplained = context.getString(R.string.attendance_widget_answered, status);
        try {
            String said = new JSONObject(answer).optString("error");
            return said.isEmpty() ? unexplained : said;
        } catch (JSONException notJSON) {
            return unexplained;
        }
    }

    private Failure unreadableAnswer(String tool, JSONException unreadable) {
        Log.w(logTag, "the widget could not read what " + tool + " answered", unreadable);
        return new Failure(context.getString(R.string.attendance_widget_unreadable), unreadable);
    }

    private static String readAll(@Nullable InputStream stream) throws IOException {
        if (stream == null) return "";
        try (InputStream reading = stream; ByteArrayOutputStream held = new ByteArrayOutputStream()) {
            byte[] chunk = new byte[4096];
            int read;
            while ((read = reading.read(chunk)) != -1) held.write(chunk, 0, read);
            return held.toString(StandardCharsets.UTF_8.name());
        }
    }

    static final class Failure extends Exception {

        Failure(String message) {
            super(message);
        }

        Failure(String message, Throwable cause) {
            super(message, cause);
        }
    }
}

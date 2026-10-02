package app.intern.kim.attendance;

import androidx.annotation.Nullable;
import java.text.ParseException;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.Locale;
import java.util.TimeZone;
import java.util.regex.Pattern;

final class CompanyClock {

    private static final Pattern fractionalSeconds = Pattern.compile("(T\\d{2}:\\d{2}:\\d{2})\\.\\d+");
    private static final String internetMoment = "yyyy-MM-dd'T'HH:mm:ssXXX";

    private CompanyClock() {}

    static String day(long moment, TimeZone timeZone) {
        return formatted(moment, "yyyy-MM-dd", timeZone);
    }

    static String time(long moment, TimeZone timeZone) {
        return formatted(moment, "HH:mm", timeZone);
    }

    static String internetString(long moment) {
        return formatted(moment, internetMoment, TimeZone.getTimeZone("UTC"));
    }

    @Nullable
    static Long moment(String written) {
        String whole = fractionalSeconds.matcher(written).replaceFirst("$1");
        try {
            Date parsed = formatter(internetMoment, TimeZone.getTimeZone("UTC")).parse(whole);
            return parsed == null ? null : parsed.getTime();
        } catch (ParseException notAMoment) {
            return null;
        }
    }

    private static String formatted(long moment, String pattern, TimeZone timeZone) {
        return formatter(pattern, timeZone).format(new Date(moment));
    }

    private static SimpleDateFormat formatter(String pattern, TimeZone timeZone) {
        SimpleDateFormat formatter = new SimpleDateFormat(pattern, Locale.US);
        formatter.setTimeZone(timeZone);
        formatter.setLenient(false);
        return formatter;
    }
}

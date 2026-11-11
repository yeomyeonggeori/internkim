package app.intern.kim.notification;

import android.util.Log;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;

final class PictureDownload {

    private static final String logTag = "PictureDownload";
    private static final int waitMilliseconds = 5000;
    private static final int largestBodyBytes = 5 * 1024 * 1024;
    private static final int chunkBytes = 16 * 1024;

    private PictureDownload() {}

    static byte[] bytesOf(String address, String pictureName) {
        if (!address.startsWith("https://")) return null;
        HttpURLConnection connection = null;
        try {
            connection = (HttpURLConnection) new URL(address).openConnection();
            connection.setConnectTimeout(waitMilliseconds);
            connection.setReadTimeout(waitMilliseconds);
            int status = connection.getResponseCode();
            if (status != HttpURLConnection.HTTP_OK) {
                Log.w(logTag, pictureName + " answered " + status);
                return null;
            }
            try (InputStream stream = connection.getInputStream()) {
                return bodyOf(stream, pictureName);
            }
        } catch (IOException failure) {
            Log.w(logTag, pictureName + " could not be fetched", failure);
            return null;
        } finally {
            if (connection != null) connection.disconnect();
        }
    }

    private static byte[] bodyOf(InputStream stream, String pictureName) throws IOException {
        ByteArrayOutputStream body = new ByteArrayOutputStream();
        byte[] chunk = new byte[chunkBytes];
        int read;
        while ((read = stream.read(chunk)) != -1) {
            if (body.size() + read > largestBodyBytes) {
                Log.w(logTag, pictureName + " is larger than " + largestBodyBytes + " bytes");
                return null;
            }
            body.write(chunk, 0, read);
        }
        return body.toByteArray();
    }
}

package app.intern.kim;

import android.content.Context;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.BitmapShader;
import android.graphics.Canvas;
import android.graphics.Matrix;
import android.graphics.Paint;
import android.graphics.Shader;
import android.util.Log;
import java.io.IOException;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;

final class SenderPicture {

    private static final String logTag = "SenderPicture";
    private static final int sidePixels = 256;
    private static final int waitMilliseconds = 5000;

    private SenderPicture() {}

    static Bitmap of(Context context, String pictureURL) {
        Bitmap fetched = fetched(pictureURL);
        Bitmap square = fetched != null ? fetched : BitmapFactory.decodeResource(context.getResources(), R.drawable.sender_default);
        return circled(square);
    }

    private static Bitmap fetched(String pictureURL) {
        if (!pictureURL.startsWith("https://")) return null;
        HttpURLConnection connection = null;
        try {
            connection = (HttpURLConnection) new URL(pictureURL).openConnection();
            connection.setConnectTimeout(waitMilliseconds);
            connection.setReadTimeout(waitMilliseconds);
            int status = connection.getResponseCode();
            if (status != HttpURLConnection.HTTP_OK) {
                Log.w(logTag, "sender picture answered " + status);
                return null;
            }
            try (InputStream stream = connection.getInputStream()) {
                Bitmap decoded = BitmapFactory.decodeStream(stream);
                if (decoded == null) Log.w(logTag, "sender picture could not be decoded");
                return decoded;
            }
        } catch (IOException failure) {
            Log.w(logTag, "sender picture could not be fetched", failure);
            return null;
        } finally {
            if (connection != null) connection.disconnect();
        }
    }

    private static Bitmap circled(Bitmap source) {
        Bitmap circle = Bitmap.createBitmap(sidePixels, sidePixels, Bitmap.Config.ARGB_8888);
        float scale = (float) sidePixels / Math.min(source.getWidth(), source.getHeight());
        Matrix fit = new Matrix();
        fit.setScale(scale, scale);
        fit.postTranslate((sidePixels - source.getWidth() * scale) / 2f, (sidePixels - source.getHeight() * scale) / 2f);
        BitmapShader shader = new BitmapShader(source, Shader.TileMode.CLAMP, Shader.TileMode.CLAMP);
        shader.setLocalMatrix(fit);
        Paint paint = new Paint(Paint.ANTI_ALIAS_FLAG);
        paint.setShader(shader);
        new Canvas(circle).drawCircle(sidePixels / 2f, sidePixels / 2f, sidePixels / 2f, paint);
        return circle;
    }
}

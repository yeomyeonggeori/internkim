package app.intern.kim.notification;

import android.content.Context;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.BitmapShader;
import android.graphics.Canvas;
import android.graphics.Matrix;
import android.graphics.Paint;
import android.graphics.Shader;
import android.util.Log;

public final class SenderPicture {

    private static final String logTag = "SenderPicture";
    private static final String pictureName = "sender picture";
    private static final int sidePixels = 256;

    private SenderPicture() {}

    public static Bitmap of(Context context, String pictureURL) {
        Bitmap fetched = fetched(pictureURL);
        Bitmap square = fetched != null ? fetched : BitmapFactory.decodeResource(context.getResources(), R.drawable.sender_default);
        return circled(square);
    }

    private static Bitmap fetched(String pictureURL) {
        byte[] bytes = PictureDownload.bytesOf(pictureURL, pictureName);
        if (bytes == null) return null;
        Bitmap decoded = BitmapFactory.decodeByteArray(bytes, 0, bytes.length);
        if (decoded == null) Log.w(logTag, pictureName + " could not be decoded");
        return decoded;
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

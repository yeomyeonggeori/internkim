package app.intern.kim.notification;

import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.util.Log;
import androidx.annotation.Nullable;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

public final class MessagePicture {

    private static final String logTag = "MessagePicture";
    private static final String pictureName = "message picture";
    private static final int longestSidePixels = 1024;
    private static final int timeLimitMilliseconds = 3000;

    private MessagePicture() {}

    @Nullable
    public static Bitmap of(String imageURL) {
        if (imageURL.isEmpty()) return null;
        byte[] bytes = fetchedInTime(imageURL);
        if (bytes == null) return null;
        Bitmap decoded = decoded(bytes);
        if (decoded == null) Log.w(logTag, pictureName + " could not be decoded");
        return decoded;
    }

    @Nullable
    private static byte[] fetchedInTime(String imageURL) {
        ExecutorService fetching = Executors.newSingleThreadExecutor();
        Future<byte[]> fetched = fetching.submit(() -> PictureDownload.bytesOf(imageURL, pictureName));
        try {
            return fetched.get(timeLimitMilliseconds, TimeUnit.MILLISECONDS);
        } catch (TimeoutException late) {
            Log.w(logTag, pictureName + " took longer than " + timeLimitMilliseconds + " milliseconds");
            return null;
        } catch (ExecutionException failure) {
            Log.w(logTag, pictureName + " could not be fetched", failure);
            return null;
        } catch (InterruptedException interrupted) {
            Thread.currentThread().interrupt();
            return null;
        } finally {
            fetching.shutdownNow();
        }
    }

    private static Bitmap decoded(byte[] bytes) {
        BitmapFactory.Options bounds = new BitmapFactory.Options();
        bounds.inJustDecodeBounds = true;
        BitmapFactory.decodeByteArray(bytes, 0, bytes.length, bounds);
        if (bounds.outWidth <= 0 || bounds.outHeight <= 0) return null;
        BitmapFactory.Options sampled = new BitmapFactory.Options();
        sampled.inSampleSize = sampleSizeFor(Math.max(bounds.outWidth, bounds.outHeight));
        return BitmapFactory.decodeByteArray(bytes, 0, bytes.length, sampled);
    }

    private static int sampleSizeFor(int longerSidePixels) {
        int sampleSize = 1;
        while (longerSidePixels / sampleSize > longestSidePixels) {
            sampleSize *= 2;
        }
        return sampleSize;
    }
}

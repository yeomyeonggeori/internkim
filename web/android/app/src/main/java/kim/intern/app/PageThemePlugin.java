package kim.intern.app;

import android.graphics.Color;
import android.view.Window;
import androidx.core.view.WindowCompat;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;

@CapacitorPlugin(name = "PageTheme")
public class PageThemePlugin extends Plugin {
    @PluginMethod
    public void apply(PluginCall call) {
        String background = call.getString("background");
        Boolean isDark = call.getBoolean("isDark");
        if (background == null || isDark == null) {
            call.reject("background and isDark are required");
            return;
        }
        int color;
        try {
            color = Color.parseColor(background);
        } catch (IllegalArgumentException notAColor) {
            call.reject("background is not a colour: " + background);
            return;
        }
        getActivity().runOnUiThread(() -> {
            Window window = getActivity().getWindow();
            window.setStatusBarColor(color);
            WindowCompat.getInsetsController(window, window.getDecorView()).setAppearanceLightStatusBars(!isDark);
            call.resolve();
        });
    }
}

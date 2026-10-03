package app.intern.kim.attendance;

import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;

@CapacitorPlugin(name = "AttendanceWidget")
public class AttendanceWidgetPlugin extends Plugin {

    @PluginMethod
    public void install(PluginCall call) {
        JSObject held = new JSObject();
        held.put("installID", AttendanceWidgetStore.installID(getContext()));
        held.put("tokenName", AttendanceWidgetStore.heldTokenName(getContext()));
        call.resolve(held);
    }

    @PluginMethod
    public void supply(PluginCall call) {
        String token = call.getString("token");
        String tokenName = call.getString("tokenName");
        String origin = call.getString("origin");
        if (token == null || tokenName == null || origin == null) {
            call.reject("token, tokenName and origin are required");
            return;
        }
        AttendanceWidgetStore.keep(getContext(), new AttendanceWidgetStore.Credential(token, tokenName, origin));
        AttendanceWidgetProvider.redrawSoon(getContext());
        call.resolve();
    }

    @PluginMethod
    public void forget(PluginCall call) {
        AttendanceWidgetStore.forget(getContext());
        AttendanceWidgetProvider.redrawSoon(getContext());
        call.resolve();
    }
}

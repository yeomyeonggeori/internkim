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
        held.put("installID", AttendanceWidgetHandoff.installID(getContext()));
        held.put("tokenName", AttendanceWidgetHandoff.heldTokenName(getContext()));
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
        AttendanceWidgetHandoff.supply(getContext(), token, tokenName, origin);
        call.resolve();
    }

    @PluginMethod
    public void forget(PluginCall call) {
        AttendanceWidgetHandoff.forget(getContext());
        call.resolve();
    }
}

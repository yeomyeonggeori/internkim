package app.intern.kim;

import android.webkit.WebView;
import androidx.activity.OnBackPressedCallback;
import androidx.activity.OnBackPressedDispatcher;

final class WebViewHistoryBack extends OnBackPressedCallback {
    private final WebView webView;
    private final OnBackPressedDispatcher dispatcher;

    WebViewHistoryBack(WebView webView, OnBackPressedDispatcher dispatcher) {
        super(true);
        this.webView = webView;
        this.dispatcher = dispatcher;
    }

    @Override
    public void handleOnBackPressed() {
        if (webView.canGoBack()) {
            webView.goBack();
            return;
        }
        setEnabled(false);
        dispatcher.onBackPressed();
        setEnabled(true);
    }
}

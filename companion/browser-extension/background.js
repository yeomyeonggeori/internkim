// Open integration point: the companion daemon binds ExtensionWebSocketBridge
// to an ephemeral port (see internal/browser/extension_bridge.go, Port()).
// Until the Chrome-launching task writes that port into runtime-config.json
// (same directory as this file) before each launch, this falls back to
// internkimDefaultBridgePort. See README.md.
const internkimDefaultBridgePort = 8787;
const internkimKeepAliveAlarmName = "internkim-bridge-keepalive";
const internkimKeepAlivePeriodMinutes = 0.4;
const internkimNavigationTimeoutMilliseconds = 30000;

let internkimSocket = null;

async function internkimResolvePort() {
  try {
    const response = await fetch(chrome.runtime.getURL("runtime-config.json"));
    if (!response.ok) return internkimDefaultBridgePort;
    const configuration = await response.json();
    return Number(configuration.port) || internkimDefaultBridgePort;
  } catch {
    return internkimDefaultBridgePort;
  }
}

async function internkimResolveSessionTabID() {
  const stored = await chrome.storage.session.get("internkimSessionTabID");
  if (typeof stored.internkimSessionTabID === "number") {
    return stored.internkimSessionTabID;
  }
  const [activeTab] = await chrome.tabs.query({ active: true, lastFocusedWindow: true });
  if (!activeTab) return null;
  await chrome.storage.session.set({ internkimSessionTabID: activeTab.id });
  return activeTab.id;
}

function internkimIsSocketOpen() {
  return internkimSocket !== null && internkimSocket.readyState === WebSocket.OPEN;
}

async function internkimEnsureConnected() {
  if (internkimIsSocketOpen()) return;
  const port = await internkimResolvePort();
  const tabID = await internkimResolveSessionTabID();
  internkimSocket = new WebSocket(`ws://127.0.0.1:${port}`);
  internkimSocket.addEventListener("open", () => internkimSendReady(tabID));
  internkimSocket.addEventListener("message", (event) => internkimHandleDaemonMessage(event.data));
  internkimSocket.addEventListener("close", () => {
    internkimSocket = null;
  });
  internkimSocket.addEventListener("error", () => {
    internkimSocket = null;
  });
}

function internkimSendReady(tabID) {
  internkimSend({
    type: "ready",
    ready: { extensionVersion: chrome.runtime.getManifest().version, tabID: tabID || 0 },
  });
}

function internkimSend(message) {
  if (!internkimIsSocketOpen()) return;
  internkimSocket.send(JSON.stringify(message));
}

async function internkimHandleDaemonMessage(rawData) {
  const message = JSON.parse(rawData);
  if (message.type === "requestSnapshot") {
    await internkimRelaySnapshotRequest(message.requestID);
    return;
  }
  if (message.type === "resolveRef") {
    await internkimRelayResolveRefRequest(message.requestID, message.ref);
    return;
  }
  if (message.type === "navigate") {
    await internkimRelayNavigateRequest(message.requestID, message.url);
  }
}

async function internkimRelaySnapshotRequest(requestID) {
  const snapshot = await internkimQueryContentScript({ internkimBridgeType: "snapshot" });
  if (!snapshot) {
    internkimSend({ type: "error", requestID, error: "no active tab reported a DOM snapshot" });
    return;
  }
  internkimSend({ type: "domSnapshot", requestID, snapshot });
}

async function internkimRelayResolveRefRequest(requestID, ref) {
  const resolvedRef = await internkimQueryContentScript({ internkimBridgeType: "resolveRef", ref });
  if (!resolvedRef) {
    internkimSend({ type: "error", requestID, error: "no active tab could resolve ref " + ref });
    return;
  }
  internkimSend({ type: "resolvedRef", requestID, resolvedRef });
}

async function internkimRelayNavigateRequest(requestID, url) {
  const tabID = await internkimResolveSessionTabID();
  if (tabID === null) {
    internkimSend({ type: "error", requestID, error: "no session tab to navigate" });
    return;
  }
  try {
    await internkimNavigateTabAndWait(tabID, url);
  } catch (navigationError) {
    internkimSend({ type: "error", requestID, error: String(navigationError.message || navigationError) });
    return;
  }
  const snapshot = await internkimQueryContentScript({ internkimBridgeType: "snapshot" });
  if (!snapshot) {
    internkimSend({ type: "error", requestID, error: "navigation completed but no snapshot was available" });
    return;
  }
  internkimSend({ type: "navigated", requestID, snapshot });
}

function internkimNavigateTabAndWait(tabID, url) {
  return new Promise((resolve, reject) => {
    const timeoutID = setTimeout(() => {
      chrome.tabs.onUpdated.removeListener(onUpdated);
      reject(new Error("navigation did not complete within " + internkimNavigationTimeoutMilliseconds + "ms"));
    }, internkimNavigationTimeoutMilliseconds);

    function onUpdated(updatedTabID, changeInfo) {
      if (updatedTabID !== tabID || changeInfo.status !== "complete") return;
      clearTimeout(timeoutID);
      chrome.tabs.onUpdated.removeListener(onUpdated);
      resolve();
    }

    chrome.tabs.onUpdated.addListener(onUpdated);
    chrome.tabs.update(tabID, { url }, () => {
      if (chrome.runtime.lastError) {
        clearTimeout(timeoutID);
        chrome.tabs.onUpdated.removeListener(onUpdated);
        reject(new Error(chrome.runtime.lastError.message));
      }
    });
  });
}

function internkimQueryContentScript(payload) {
  return new Promise((resolve) => {
    internkimResolveSessionTabID().then((tabID) => {
      if (tabID === null) {
        resolve(null);
        return;
      }
      chrome.tabs.sendMessage(tabID, payload, { frameId: 0 }, (response) => {
        if (chrome.runtime.lastError) {
          resolve(null);
          return;
        }
        resolve(response || null);
      });
    });
  });
}

chrome.runtime.onMessage.addListener((message) => {
  if (message.internkimBridgeType === "domChanged") {
    chrome.storage.session.set({ internkimLastSnapshot: message.snapshot });
  }
});

chrome.alarms.create(internkimKeepAliveAlarmName, { periodInMinutes: internkimKeepAlivePeriodMinutes });
chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === internkimKeepAliveAlarmName) internkimEnsureConnected();
});

chrome.runtime.onStartup.addListener(internkimEnsureConnected);
chrome.runtime.onInstalled.addListener(internkimEnsureConnected);
internkimEnsureConnected();

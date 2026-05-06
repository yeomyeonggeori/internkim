const rootID = 'internkim-companion-handoff-root';
const readyID = 'internkim-companion-extension-ready';
let currentHandoff;

markExtensionReady();
setInterval(refreshHandoff, 1000);
void refreshHandoff();

async function refreshHandoff() {
  markExtensionReady();
  const state = await sendMessage({ type: 'internkim.handoff.state' }).catch(() => undefined);
  if (!state?.active) {
    removeOverlay();
    currentHandoff = undefined;
    return;
  }
  currentHandoff = state;
  renderOverlay(state);
}

function markExtensionReady() {
  if (document.getElementById(readyID)) return;
  const marker = document.createElement('meta');
  marker.id = readyID;
  marker.dataset.source = 'internkim-companion-extension';
  document.documentElement.appendChild(marker);
}

function renderOverlay(state) {
  const host = ensureHost();
  const root = host.shadowRoot;
  const message = escapeHTML(state.message || '필요한 작업을 마친 뒤 완료를 눌러주세요.');
  root.innerHTML = `
    <style>
      :host {
        all: initial;
      }
      .wrap {
        position: fixed;
        top: 18px;
        left: 50%;
        z-index: 2147483647;
        display: flex;
        transform: translateX(-50%);
        align-items: center;
        gap: 10px;
        max-width: min(520px, calc(100vw - 32px));
        border: 1px solid rgba(15, 23, 42, 0.12);
        border-radius: 8px;
        background: rgba(255, 255, 255, 0.96);
        box-shadow: 0 12px 36px rgba(15, 23, 42, 0.14);
        padding: 8px 8px 8px 12px;
        color: rgb(15, 23, 42);
        font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      }
      .message {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-size: 13px;
        line-height: 20px;
        font-weight: 500;
      }
      button {
        display: inline-flex;
        height: 36px;
        flex-shrink: 0;
        align-items: center;
        justify-content: center;
        border: 1px solid transparent;
        border-radius: 6px;
        background: rgb(15, 23, 42);
        padding: 0 14px;
        color: white;
        font-size: 14px;
        line-height: 20px;
        font-weight: 500;
        white-space: nowrap;
        cursor: pointer;
        transition: background 120ms ease, transform 120ms ease, box-shadow 120ms ease;
      }
      button:hover {
        background: rgb(30, 41, 59);
      }
      button:active {
        transform: translateY(1px);
      }
      button:focus-visible {
        outline: none;
        box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.45);
      }
    </style>
    <div class="wrap" role="region" aria-label="Intern Kim browser handoff">
      <div class="message">${message}</div>
      <button type="button">완료</button>
    </div>
  `;
  root.querySelector('button')?.addEventListener('click', completeHandoff, { once: true });
}

function ensureHost() {
  const existingHost = document.getElementById(rootID);
  if (existingHost?.shadowRoot) return existingHost;
  const host = document.createElement('div');
  host.id = rootID;
  host.attachShadow({ mode: 'open' });
  document.documentElement.appendChild(host);
  return host;
}

function removeOverlay() {
  document.getElementById(rootID)?.remove();
}

async function completeHandoff() {
  if (!currentHandoff) return;
  await sendMessage({
    type: 'internkim.handoff.complete',
    handoffID: currentHandoff.handoffID,
    sessionID: currentHandoff.sessionID,
    url: location.href,
    title: document.title
  }).catch(() => undefined);
}

function sendMessage(message) {
  return new Promise((resolve) => {
    chrome.runtime.sendMessage(message, resolve);
  });
}

function escapeHTML(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;');
}

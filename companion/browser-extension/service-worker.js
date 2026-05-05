const bridgeURL = 'http://127.0.0.1:7983/v1/browser/handoff';

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message?.type === 'internkim.handoff.state') {
    fetchState().then(sendResponse);
    return true;
  }
  if (message?.type === 'internkim.handoff.complete') {
    completeHandoff(message, sender).then(sendResponse);
    return true;
  }
  return false;
});

async function fetchState() {
  try {
    const response = await fetch(bridgeURL, { cache: 'no-store' });
    if (!response.ok) return { active: false };
    return await response.json();
  } catch {
    return { active: false };
  }
}

async function completeHandoff(message, sender) {
  const tabURL = message.url || sender?.tab?.url || '';
  const response = await fetch(`${bridgeURL}/complete`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      handoffID: message.handoffID,
      sessionID: message.sessionID,
      url: tabURL,
      title: message.title || ''
    })
  });
  return { ok: response.ok, error: response.ok ? '' : await response.text() };
}

const internkimRefAttribute = "data-internkim-ref";
const internkimMaxElements = 300;
const internkimMutationDebounceMilliseconds = 500;

let internkimRefCounter = 0;
let internkimMutationTimer = null;

function internkimNextRef() {
  internkimRefCounter += 1;
  return "e" + internkimRefCounter;
}

function internkimIsVisible(rect, style) {
  return rect.width > 0 && rect.height > 0 && style.visibility !== "hidden" && style.display !== "none";
}

function internkimInteractiveCandidates() {
  const selector = 'a, button, input, select, textarea, [role="button"], [role="link"], [onclick], [tabindex]';
  const candidates = [];
  for (const element of document.querySelectorAll(selector)) {
    if (candidates.length >= internkimMaxElements) break;
    const tabIndexAttribute = element.getAttribute("tabindex");
    if (tabIndexAttribute !== null && Number(tabIndexAttribute) < 0) continue;
    const rect = element.getBoundingClientRect();
    if (!internkimIsVisible(rect, getComputedStyle(element))) continue;
    candidates.push({ element, rect });
  }
  return candidates;
}

function internkimElementText(element) {
  const text = element.innerText || element.value || element.getAttribute("aria-label") || "";
  return text.trim().slice(0, 200);
}

function internkimDescribeElement(element, rect) {
  let ref = element.getAttribute(internkimRefAttribute);
  if (!ref) {
    ref = internkimNextRef();
    element.setAttribute(internkimRefAttribute, ref);
  }
  return {
    ref,
    tag: element.tagName.toLowerCase(),
    role: element.getAttribute("role") || "",
    text: internkimElementText(element),
    rect: { x: rect.x, y: rect.y, width: rect.width, height: rect.height },
  };
}

function internkimViewportGeometry() {
  return {
    screenX: window.screenX,
    screenY: window.screenY,
    devicePixelRatio: window.devicePixelRatio,
    outerWidth: window.outerWidth,
    innerWidth: window.innerWidth,
    outerHeight: window.outerHeight,
    innerHeight: window.innerHeight,
  };
}

function internkimSnapshot() {
  return {
    url: window.location.href,
    title: document.title,
    elements: internkimInteractiveCandidates().map(({ element, rect }) => internkimDescribeElement(element, rect)),
    viewport: internkimViewportGeometry(),
  };
}

function internkimResolveRef(ref) {
  const element = document.querySelector(`[${internkimRefAttribute}="${CSS.escape(ref)}"]`);
  if (!element) {
    return { ref, found: false, rect: { x: 0, y: 0, width: 0, height: 0 }, viewport: internkimViewportGeometry() };
  }
  const rect = element.getBoundingClientRect();
  return {
    ref,
    found: true,
    rect: { x: rect.x, y: rect.y, width: rect.width, height: rect.height },
    viewport: internkimViewportGeometry(),
  };
}

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message.internkimBridgeType === "snapshot") {
    sendResponse(internkimSnapshot());
    return false;
  }
  if (message.internkimBridgeType === "resolveRef") {
    sendResponse(internkimResolveRef(message.ref));
    return false;
  }
  return false;
});

// Only the top frame reports background changes; cross-frame ref resolution is a known follow-up, not this task's scope.
if (window === window.top) {
  const internkimMutationObserver = new MutationObserver(() => {
    if (internkimMutationTimer) clearTimeout(internkimMutationTimer);
    internkimMutationTimer = setTimeout(() => {
      chrome.runtime.sendMessage({ internkimBridgeType: "domChanged", snapshot: internkimSnapshot() });
    }, internkimMutationDebounceMilliseconds);
  });
  internkimMutationObserver.observe(document.documentElement, { childList: true, subtree: true, attributes: true });
}

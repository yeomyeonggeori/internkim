const MOBILE_KEYBOARD_MIN_HEIGHT = 120;
const MOBILE_KEYBOARD_OPEN_ATTRIBUTE = 'data-mobile-keyboard-open';
const VIEWPORT_PROPERTIES = ['--app-viewport-height', '--app-viewport-bottom', '--app-viewport-top', '--app-viewport-safe-bottom'];

function forgetVisualViewport(root: HTMLElement): void {
	for (const property of VIEWPORT_PROPERTIES) root.style.removeProperty(property);
	root.removeAttribute(MOBILE_KEYBOARD_OPEN_ATTRIBUTE);
}

function followVisualViewport(root: HTMLElement, viewport: VisualViewport, keyboardHeight: number): void {
	root.style.setProperty('--app-viewport-height', `${viewport.height}px`);
	root.style.setProperty('--app-viewport-top', `${viewport.offsetTop}px`);
	root.style.setProperty('--app-viewport-bottom', `${keyboardHeight}px`);
	root.style.setProperty('--app-viewport-safe-bottom', '0px');
	root.setAttribute(MOBILE_KEYBOARD_OPEN_ATTRIBUTE, '');
}

export function keepAppInVisualViewport(): () => void {
	const viewport = window.visualViewport;
	if (!viewport) return () => {};
	const root = document.documentElement;
	let wasKeyboardOpen = false;
	const updateViewport = () => {
		const keyboardHeight = Math.max(0, window.innerHeight - viewport.height - viewport.offsetTop);
		const isKeyboardOpen = window.innerWidth < 640 && viewport.scale === 1 && keyboardHeight >= MOBILE_KEYBOARD_MIN_HEIGHT;
		if (isKeyboardOpen) followVisualViewport(root, viewport, keyboardHeight);
		else forgetVisualViewport(root);
		if (wasKeyboardOpen && !isKeyboardOpen && window.scrollY !== 0) window.scrollTo(0, 0);
		wasKeyboardOpen = isKeyboardOpen;
	};
	viewport.addEventListener('resize', updateViewport);
	viewport.addEventListener('scroll', updateViewport);
	window.addEventListener('resize', updateViewport);
	updateViewport();
	return () => {
		viewport.removeEventListener('resize', updateViewport);
		viewport.removeEventListener('scroll', updateViewport);
		window.removeEventListener('resize', updateViewport);
		forgetVisualViewport(root);
	};
}

import { followNativeKeyboard, type NativeKeyboardWatcher } from './native-shell/keyboard';

const MOBILE_KEYBOARD_MIN_HEIGHT = 120;
const MOBILE_KEYBOARD_OPEN_ATTRIBUTE = 'data-mobile-keyboard-open';
const VIEWPORT_PROPERTIES = ['--app-viewport-height', '--app-viewport-bottom', '--app-viewport-top', '--app-viewport-safe-bottom'];

function forgetVisualViewport(root: HTMLElement): void {
	for (const property of VIEWPORT_PROPERTIES) root.style.removeProperty(property);
	root.removeAttribute(MOBILE_KEYBOARD_OPEN_ATTRIBUTE);
}

function markKeyboardOpen(root: HTMLElement): void {
	root.style.setProperty('--app-viewport-safe-bottom', '0px');
	root.setAttribute(MOBILE_KEYBOARD_OPEN_ATTRIBUTE, '');
}

function followVisualViewport(root: HTMLElement, viewport: VisualViewport, keyboardHeight: number): void {
	root.style.setProperty('--app-viewport-height', `${viewport.height}px`);
	root.style.setProperty('--app-viewport-top', `${viewport.offsetTop}px`);
	root.style.setProperty('--app-viewport-bottom', `${keyboardHeight}px`);
	markKeyboardOpen(root);
}

export function keepAppInVisualViewport(watchNativeKeyboard: NativeKeyboardWatcher = followNativeKeyboard): () => void {
	const viewport = window.visualViewport;
	if (!viewport) return () => {};
	const root = document.documentElement;
	let wasKeyboardOpen = false;
	let isNativeKeyboardOpen = false;
	const updateViewport = () => {
		const keyboardHeight = Math.max(0, window.innerHeight - viewport.height - viewport.offsetTop);
		const isPhone = window.innerWidth < 640;
		const coversVisualViewport = isPhone && viewport.scale === 1 && keyboardHeight >= MOBILE_KEYBOARD_MIN_HEIGHT;
		const isKeyboardOpen = coversVisualViewport || (isPhone && isNativeKeyboardOpen);
		if (coversVisualViewport) followVisualViewport(root, viewport, keyboardHeight);
		else forgetVisualViewport(root);
		if (!coversVisualViewport && isKeyboardOpen) markKeyboardOpen(root);
		if (wasKeyboardOpen && !isKeyboardOpen && window.scrollY !== 0) window.scrollTo(0, 0);
		wasKeyboardOpen = isKeyboardOpen;
	};
	viewport.addEventListener('resize', updateViewport);
	viewport.addEventListener('scroll', updateViewport);
	window.addEventListener('resize', updateViewport);
	const stopWatchingNativeKeyboard = watchNativeKeyboard((isOpen) => {
		isNativeKeyboardOpen = isOpen;
		updateViewport();
	});
	updateViewport();
	return () => {
		stopWatchingNativeKeyboard();
		viewport.removeEventListener('resize', updateViewport);
		viewport.removeEventListener('scroll', updateViewport);
		window.removeEventListener('resize', updateViewport);
		forgetVisualViewport(root);
	};
}

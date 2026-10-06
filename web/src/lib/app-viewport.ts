const MOBILE_KEYBOARD_MIN_HEIGHT = 120;
const MOBILE_KEYBOARD_OPEN_ATTRIBUTE = 'data-mobile-keyboard-open';

export function keepAppInVisualViewport(): () => void {
	const viewport = window.visualViewport;
	if (!viewport) return () => {};
	const root = document.documentElement;
	const updateViewport = () => {
		if (window.innerWidth >= 640 || viewport.scale !== 1) {
			root.style.removeProperty('--app-viewport-height');
			root.style.removeProperty('--app-viewport-bottom');
			root.style.removeProperty('--app-viewport-top');
			root.style.removeProperty('--app-viewport-safe-bottom');
			root.removeAttribute(MOBILE_KEYBOARD_OPEN_ATTRIBUTE);
			return;
		}
		const viewportBottom = Math.max(0, window.innerHeight - viewport.height - viewport.offsetTop);
		root.style.setProperty('--app-viewport-height', `${viewport.height}px`);
		root.style.setProperty('--app-viewport-top', `${viewport.offsetTop}px`);
		root.style.setProperty('--app-viewport-bottom', `${viewportBottom}px`);
		root.style.setProperty('--app-viewport-safe-bottom', viewportBottom > 0 ? '0px' : 'var(--app-mobile-nav-bottom)');
		root.toggleAttribute(MOBILE_KEYBOARD_OPEN_ATTRIBUTE, viewportBottom >= MOBILE_KEYBOARD_MIN_HEIGHT);
	};
	viewport.addEventListener('resize', updateViewport);
	viewport.addEventListener('scroll', updateViewport);
	window.addEventListener('resize', updateViewport);
	updateViewport();
	return () => {
		viewport.removeEventListener('resize', updateViewport);
		viewport.removeEventListener('scroll', updateViewport);
		window.removeEventListener('resize', updateViewport);
		root.style.removeProperty('--app-viewport-height');
		root.style.removeProperty('--app-viewport-bottom');
		root.style.removeProperty('--app-viewport-top');
		root.style.removeProperty('--app-viewport-safe-bottom');
		root.removeAttribute(MOBILE_KEYBOARD_OPEN_ATTRIBUTE);
	};
}

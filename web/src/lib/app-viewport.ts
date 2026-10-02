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
			return;
		}
		root.style.setProperty('--app-viewport-height', `${viewport.height}px`);
		root.style.setProperty('--app-viewport-top', `${viewport.offsetTop}px`);
		root.style.setProperty('--app-viewport-bottom', `${Math.max(0, window.innerHeight - viewport.height - viewport.offsetTop)}px`);
		root.style.setProperty('--app-viewport-safe-bottom', window.innerHeight > viewport.height + viewport.offsetTop ? '0px' : 'var(--app-mobile-nav-bottom)');
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
	};
}

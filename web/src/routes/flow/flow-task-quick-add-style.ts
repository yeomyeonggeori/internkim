const panelBaseClass = [
	'fixed bottom-[calc(9.75rem+env(safe-area-inset-bottom))] right-[max(0.75rem,calc((100vw-30rem)/2))]',
	'z-[55] flex h-[min(500px,calc(100vh-10rem))] w-[calc(100vw-2rem)] max-w-[400px] flex-col overflow-hidden',
	'rounded-2xl bg-popover text-popover-foreground shadow-2xl ring-1 ring-foreground/10',
	'transition-all duration-[400ms] sm:bottom-[6.25rem] sm:right-6 sm:h-[min(500px,calc(100vh-7rem))]'
].join(' ');

export function quickAddPanelClass(isVisible: boolean): string {
	const visibilityClass = isVisible
		? 'translate-y-0 scale-100 opacity-100'
		: 'pointer-events-none translate-y-3 scale-95 opacity-0';
	return `${panelBaseClass} ${visibilityClass}`;
}

export function quickAddOverlayClass(isVisible: boolean): string {
	const visibilityClass = isVisible ? 'opacity-100' : 'pointer-events-none opacity-0';
	return `fixed inset-0 z-50 cursor-default bg-black/10 transition-opacity duration-[400ms] supports-backdrop-filter:backdrop-blur-xs ${visibilityClass}`;
}

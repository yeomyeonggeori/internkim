const launcherAnchorClass = 'fixed bottom-[calc(5.75rem+env(safe-area-inset-bottom))] right-[max(0.75rem,calc((100vw-30rem)/2))] md:bottom-6 md:right-6';
const launcherTransitionClass = 'transition-all duration-[400ms]';
const launcherContentBaseClass = [
	'absolute inset-0 flex items-center justify-center gap-1.5',
	launcherTransitionClass
].join(' ');
const panelBaseClass = [
	'fixed bottom-[calc(8.75rem+env(safe-area-inset-bottom))] right-[max(0.75rem,calc((100vw-30rem)/2))]',
	'z-[55] flex h-[min(500px,calc(100vh-10rem))] w-[calc(100vw-2rem)] max-w-[400px] flex-col overflow-hidden',
	'rounded-2xl bg-popover text-popover-foreground shadow-2xl ring-1 ring-foreground/10',
	'transition-all duration-[400ms] md:bottom-[6.25rem] md:right-6 md:h-[min(500px,calc(100vh-7rem))]'
].join(' ');

export function quickAddLauncherClass(isOpen: boolean): string {
	if (isOpen) {
		return [
			launcherAnchorClass,
			launcherTransitionClass,
			'z-[60] size-12 rounded-full bg-destructive px-0 text-destructive-foreground shadow-xl hover:bg-destructive/90'
		].join(' ');
	}
	return [
		launcherAnchorClass,
		launcherTransitionClass,
		'z-30 h-10 w-36 rounded-full px-3 text-sm shadow-lg md:z-[60] md:px-4'
	].join(' ');
}

export function quickAddLauncherContentClass(isVisible: boolean): string {
	const visibilityClass = isVisible ? 'opacity-100 scale-100' : 'opacity-0 scale-75';
	return `${launcherContentBaseClass} ${visibilityClass}`;
}

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

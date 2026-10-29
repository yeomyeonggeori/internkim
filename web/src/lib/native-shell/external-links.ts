import { openInDefaultBrowser } from './external-links-tauri';
import { isInsideTauriShell } from './shell';

type LinkAddress = { protocol: string; origin: string };

const browserProtocols = ['http:', 'https:'];
const handedOffProtocols = ['mailto:', 'tel:'];

export function leavesTheApp(link: LinkAddress, appOrigin: string): boolean {
	if (handedOffProtocols.includes(link.protocol)) return true;
	return browserProtocols.includes(link.protocol) && link.origin !== appOrigin;
}

export function openLinksLeavingTheAppInBrowser(): () => void {
	if (!isInsideTauriShell()) return () => {};
	const openInBrowser = (event: MouseEvent) => {
		if (event.defaultPrevented || event.button !== 0) return;
		const link = clickedLink(event);
		if (!link || !leavesTheApp(link, window.location.origin)) return;
		event.preventDefault();
		openInDefaultBrowser(link.href).catch((failure: unknown) =>
			console.warn('the link did not open in the browser', failure)
		);
	};
	document.addEventListener('click', openInBrowser);
	return () => document.removeEventListener('click', openInBrowser);
}

function clickedLink(event: MouseEvent): HTMLAnchorElement | null {
	for (const target of event.composedPath()) {
		if (target instanceof HTMLAnchorElement && target.href) return target;
	}
	return null;
}

import { page } from '$app/state';
import { appNavigation } from '$lib/components/app-navigation.svelte';
import { routePathOf } from '$lib/company-path';
import { allowBackGesture } from '$lib/native-shell/back-gesture';

function withoutTrailingSlash(path: string): string {
	return path.length > 1 ? path.replace(/\/$/, '') : path;
}

function isOnMainPage(): boolean {
	if (Object.keys(page.state).length > 0) return false;
	const currentPath = withoutTrailingSlash(appNavigation.currentPath);
	return [...appNavigation.apps, ...appNavigation.workspace].some(
		(item) => withoutTrailingSlash(routePathOf(item.href)) === currentPath
	);
}

export function keepBackGestureOffOnMainPages(): () => void {
	return $effect.root(() => {
		$effect(() => {
			const isAllowed = !isOnMainPage();
			allowBackGesture(isAllowed).catch((failure: unknown) =>
				console.warn('the app did not take the back gesture choice', { isAllowed }, failure)
			);
		});
	});
}

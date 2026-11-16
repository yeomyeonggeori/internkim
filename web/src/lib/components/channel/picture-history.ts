import { pushState } from '$app/navigation';
import { page } from '$app/state';

export function isPictureOpenInHistory(): boolean {
	return page.state.isPictureOpen === true;
}

export function rememberOpenedPicture(): void {
	if (isPictureOpenInHistory()) return;
	pushState('', { ...page.state, isPictureOpen: true });
}

export function forgetOpenedPicture(): void {
	if (isPictureOpenInHistory()) history.back();
}

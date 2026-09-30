const formatToolbarShownKey = 'internkim.messenger.format-toolbar-shown';
const shownValue = '1';

export function isFormatToolbarShown(): boolean {
	try {
		return window.localStorage.getItem(formatToolbarShownKey) === shownValue;
	} catch {
		return false;
	}
}

export function rememberFormatToolbarShown(isShown: boolean): void {
	try {
		if (isShown) window.localStorage.setItem(formatToolbarShownKey, shownValue);
		else window.localStorage.removeItem(formatToolbarShownKey);
	} catch {
		return;
	}
}

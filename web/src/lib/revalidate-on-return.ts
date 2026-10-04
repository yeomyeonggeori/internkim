/** Focus and visibility usually describe the same return to the app. */
export function revalidateOnReturn(
	windowEvents: Pick<Window, 'addEventListener' | 'removeEventListener'>,
	documentEvents: Pick<Document, 'addEventListener' | 'removeEventListener' | 'visibilityState' | 'hasFocus'>,
	revalidate: () => void
): () => void {
	let needsValidation = true;
	const leave = () => { needsValidation = true; };
	const returnToApp = () => {
		if (documentEvents.visibilityState !== 'visible' || !needsValidation) return;
		needsValidation = false;
		revalidate();
	};
	const visibilityChanged = () => {
		if (documentEvents.visibilityState !== 'visible') leave();
		else {
			returnToApp();
			// A visible background window may receive focus much later.
			if (!documentEvents.hasFocus()) needsValidation = true;
		}
	};
	windowEvents.addEventListener('blur', leave);
	windowEvents.addEventListener('focus', returnToApp);
	documentEvents.addEventListener('visibilitychange', visibilityChanged);
	return () => {
		windowEvents.removeEventListener('blur', leave);
		windowEvents.removeEventListener('focus', returnToApp);
		documentEvents.removeEventListener('visibilitychange', visibilityChanged);
	};
}

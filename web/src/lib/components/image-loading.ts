export type ImageLoadState = 'loading' | 'loaded' | 'error';

export function observeImageLoad(image: Pick<HTMLImageElement, 'complete' | 'naturalWidth'> & EventTarget, onState: (state: ImageLoadState) => void) {
	let active = true;
	const loaded = () => {
		if (active) onState(image.naturalWidth > 0 ? 'loaded' : 'error');
	};
	const failed = () => {
		if (active) onState('error');
	};
	image.addEventListener('load', loaded);
	image.addEventListener('error', failed);
	if (image.complete) loaded();

	return {
		destroy() {
			active = false;
			image.removeEventListener('load', loaded);
			image.removeEventListener('error', failed);
		}
	};
}

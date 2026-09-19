export type MessageCopy =
	| { kind: 'text'; text: string }
	| { kind: 'picture'; address: string }
	| { kind: 'nothing' };

export type CopyOutcome = 'success' | 'failure';

export function whatToCopy(visibleText: string, pictureAddresses: string[]): MessageCopy {
	if (visibleText !== '') return { kind: 'text', text: visibleText };
	const firstPicture = pictureAddresses[0];
	if (firstPicture) return { kind: 'picture', address: firstPicture };
	return { kind: 'nothing' };
}

function redrawnAsPNG(picture: ImageBitmap): Promise<Blob> {
	const canvas = document.createElement('canvas');
	canvas.width = picture.width;
	canvas.height = picture.height;
	const context = canvas.getContext('2d');
	if (!context) throw new Error('this browser gave no canvas to redraw the picture on');
	context.drawImage(picture, 0, 0);
	return new Promise((resolve, reject) => {
		canvas.toBlob((redrawn) => {
			if (redrawn) resolve(redrawn);
			else reject(new Error('the picture could not be redrawn as PNG'));
		}, 'image/png');
	});
}

async function pictureAsPNG(address: string): Promise<Blob> {
	const answer = await fetch(address);
	if (!answer.ok) throw new Error(`the picture did not arrive: HTTP ${answer.status}`);
	const arrived = await answer.blob();
	if (arrived.type === 'image/png') return arrived;
	const decoded = await createImageBitmap(arrived);
	try {
		return await redrawnAsPNG(decoded);
	} finally {
		decoded.close();
	}
}

export async function copyPicture(address: string): Promise<CopyOutcome> {
	if (!navigator.clipboard?.write || typeof ClipboardItem === 'undefined') return 'failure';
	try {
		await navigator.clipboard.write([new ClipboardItem({ 'image/png': pictureAsPNG(address) })]);
		return 'success';
	} catch (failure) {
		console.warn('the picture was not put on the clipboard', failure);
		return 'failure';
	}
}

import { startDownload } from './attachment-download';
import type { CopyOutcome } from './message-copy';

const capturePixelRatio = 2;

function rowsInOrder(scroller: HTMLElement, messageIDs: string[]): HTMLElement[] {
	return messageIDs.flatMap((messageID) => {
		const row = scroller.querySelector<HTMLElement>(`[data-message-id="${CSS.escape(messageID)}"]`);
		return row ? [row] : [];
	});
}

function copyWithPaintedCanvases(row: HTMLElement): Node {
	const copy = row.cloneNode(true);
	if (!(copy instanceof HTMLElement)) return copy;
	const painted = row.querySelectorAll('canvas');
	copy.querySelectorAll('canvas').forEach((canvas, index) => {
		const source = painted[index];
		if (!source || source.width === 0 || source.height === 0) return;
		canvas.width = source.width;
		canvas.height = source.height;
		canvas.getContext('2d')?.drawImage(source, 0, 0);
	});
	return copy;
}

function stagedCopyOf(scroller: HTMLElement, rows: HTMLElement[]): HTMLElement {
	const stage = document.createElement('div');
	stage.className = '@container/conversation bg-background text-foreground flex flex-col gap-4 px-4 py-4';
	stage.style.cssText = `position: fixed; top: 0; left: -100000px; width: ${scroller.clientWidth}px;`;
	let group: HTMLElement | null = null;
	let groupSource: Element | null = null;
	for (const row of rows) {
		if (row.parentElement !== groupSource || group === null) {
			groupSource = row.parentElement;
			group = document.createElement('div');
			group.className = groupSource?.className ?? '';
			stage.appendChild(group);
		}
		group.appendChild(copyWithPaintedCanvases(row));
	}
	document.body.appendChild(stage);
	return stage;
}

export async function messagesAsPNG(scroller: HTMLElement, messageIDs: string[]): Promise<Blob> {
	const rows = rowsInOrder(scroller, messageIDs);
	if (rows.length === 0) throw new Error('none of the chosen messages is on screen to capture');
	const stage = stagedCopyOf(scroller, rows);
	try {
		const { snapdom } = await import('@zumer/snapdom');
		return await snapdom.toBlob(stage, {
			format: 'png',
			dpr: capturePixelRatio,
			backgroundColor: getComputedStyle(stage).backgroundColor
		});
	} finally {
		stage.remove();
	}
}

export async function copyCaptureImage(picture: Promise<Blob>): Promise<CopyOutcome> {
	if (!navigator.clipboard?.write || typeof ClipboardItem === 'undefined') return 'failure';
	try {
		await navigator.clipboard.write([new ClipboardItem({ 'image/png': picture })]);
		return 'success';
	} catch (failure) {
		console.warn('the captured messages were not put on the clipboard', failure);
		return 'failure';
	}
}

function captureFilename(now: Date): string {
	const pad = (value: number): string => String(value).padStart(2, '0');
	const day = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
	return `conversation-${day}-${pad(now.getHours())}${pad(now.getMinutes())}.png`;
}

export async function saveCaptureImage(picture: Promise<Blob>): Promise<CopyOutcome> {
	try {
		const address = URL.createObjectURL(await picture);
		startDownload(address, captureFilename(new Date()));
		setTimeout(() => URL.revokeObjectURL(address), 0);
		return 'success';
	} catch (failure) {
		console.warn('the captured messages were not saved', failure);
		return 'failure';
	}
}

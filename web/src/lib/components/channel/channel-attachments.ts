import type { ChannelOutgoingAttachment } from './channel-api';

export function formatAttachmentMeta(attachment: {
	mimeType?: string;
	filename?: string;
	sizeBytes?: number;
}): string {
	const parts: string[] = [];
	const typeLabel = attachmentTypeLabel(attachment.mimeType, attachment.filename);
	if (typeLabel) parts.push(typeLabel);
	if (attachment.sizeBytes && attachment.sizeBytes > 0) parts.push(formatBytes(attachment.sizeBytes));
	return parts.join(' · ');
}

function attachmentTypeLabel(mimeType?: string, filename?: string): string {
	const subtype = mimeType?.split('/')[1];
	if (subtype) return subtype.toUpperCase();
	const extension = filename?.split('.').pop();
	return extension && extension !== filename ? extension.toUpperCase() : '';
}

function formatBytes(bytes: number): string {
	if (bytes < 1024) return `${bytes} B`;
	if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
	return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

const maxImageDimension = 2048;
const reencodedImageTypes = new Set(['image/png', 'image/jpeg', 'image/webp']);

export async function fileToAttachment(file: File): Promise<ChannelOutgoingAttachment> {
	if (reencodedImageTypes.has(file.type)) return reencodeImage(file);
	const contentBase64 = await blobToBase64(file);
	return {
		filename: file.name,
		contentType: file.type || 'application/octet-stream',
		contentBase64
	};
}

async function reencodeImage(file: File): Promise<ChannelOutgoingAttachment> {
	const bitmap = await createImageBitmap(file);
	const scale = Math.min(1, maxImageDimension / Math.max(bitmap.width, bitmap.height));
	const width = Math.max(1, Math.round(bitmap.width * scale));
	const height = Math.max(1, Math.round(bitmap.height * scale));
	const canvas = document.createElement('canvas');
	canvas.width = width;
	canvas.height = height;
	const context = canvas.getContext('2d');
	if (!context) throw new Error('canvas 2d context unavailable');
	context.drawImage(bitmap, 0, 0, width, height);
	bitmap.close();
	const outputType = file.type === 'image/png' ? 'image/png' : 'image/jpeg';
	const blob = await canvasToBlob(canvas, outputType);
	return {
		filename: withExtension(file.name, outputType),
		contentType: outputType,
		contentBase64: await blobToBase64(blob)
	};
}

function canvasToBlob(canvas: HTMLCanvasElement, type: string): Promise<Blob> {
	return new Promise((resolve, reject) => {
		canvas.toBlob(
			(blob) => (blob ? resolve(blob) : reject(new Error('canvas encode failed'))),
			type,
			0.9
		);
	});
}

function blobToBase64(blob: Blob): Promise<string> {
	return new Promise((resolve, reject) => {
		const reader = new FileReader();
		reader.onload = () => {
			const result = reader.result;
			if (typeof result !== 'string') return reject(new Error('unexpected file read result'));
			resolve(result.slice(result.indexOf(',') + 1));
		};
		reader.onerror = () => reject(reader.error ?? new Error('file read failed'));
		reader.readAsDataURL(blob);
	});
}

function withExtension(filename: string, contentType: string): string {
	const extension = contentType === 'image/png' ? 'png' : 'jpg';
	const base = filename.replace(/\.[^.]+$/, '');
	return `${base || 'image'}.${extension}`;
}

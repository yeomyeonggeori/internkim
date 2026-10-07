import type { ChannelMessageAttachment, ChannelOutgoingAttachment } from './channel-api';
import type { AttachmentState } from '$lib/components/ui/attachment/index.js';
import type { AttachmentProgress, AttachmentSourceStatus } from '$lib/stores/attachment-source.svelte';
import type { LightboxItem } from './channel-lightbox.svelte';

export function openableAttachments(
	attachments: ChannelMessageAttachment[],
	openableAddressOf: (url: string) => string
): ChannelMessageAttachment[] {
	return attachments.map((attachment) => ({
		...attachment,
		source: attachment.source || openableAddressOf(attachment.url)
	}));
}

export function attachmentStateOf(source: string | undefined, status: AttachmentSourceStatus): AttachmentState {
	if (source) return 'done';
	if (status === 'loading') return 'processing';
	if (status === 'failed') return 'error';
	return 'done';
}

export function preparingLabel(label: string, progress: AttachmentProgress | null): string {
	if (!progress || progress.totalBytes <= 0) return label;
	return `${label} ${Math.floor((progress.copiedBytes / progress.totalBytes) * 100)}%`;
}

export function pictureAddressesOf(attachments: ChannelMessageAttachment[]): string[] {
	return attachments
		.filter((attachment) => attachment.kind === 'image' && attachment.source)
		.map((attachment) => attachment.source ?? '');
}

export type MessagePicture = { address: string; filename: string };

export function messagePicturesOf(attachments: ChannelMessageAttachment[]): MessagePicture[] {
	return attachments
		.filter((attachment) => attachment.kind === 'image' && attachment.source)
		.map((attachment) => ({ address: attachment.source ?? '', filename: attachment.filename ?? '' }));
}

export function lightboxItemsOf(attachments: ChannelMessageAttachment[]): LightboxItem[] {
	return attachments.flatMap((attachment) => {
		if (!attachment.source) return [];
		if (attachment.kind !== 'image' && attachment.kind !== 'video') return [];
		return [
			{
				kind: attachment.kind,
				source: attachment.source,
				filename: attachment.filename ?? '',
				width: attachment.widthPixels,
				height: attachment.heightPixels
			}
		];
	});
}

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
	return {
		filename: file.name,
		contentType: file.type || 'application/octet-stream',
		content: file
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
		content: blob
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

function withExtension(filename: string, contentType: string): string {
	const extension = contentType === 'image/png' ? 'png' : 'jpg';
	const base = filename.replace(/\.[^.]+$/, '');
	return `${base || 'image'}.${extension}`;
}

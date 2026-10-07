export type AttachmentKind = 'image' | 'video' | 'file';

export function attachmentKindOf(contentType: string): AttachmentKind {
	if (contentType.startsWith('image/')) return 'image';
	if (contentType.startsWith('video/')) return 'video';
	return 'file';
}

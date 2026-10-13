type ClipboardContents = Pick<DataTransfer, 'files' | 'getData'>;

export function filesToAttachFromClipboard(clipboard: ClipboardContents | null): File[] {
	if (!clipboard || clipboard.files.length === 0) return [];
	if (clipboard.getData('text/plain').trim() !== '') return [];
	return Array.from(clipboard.files);
}

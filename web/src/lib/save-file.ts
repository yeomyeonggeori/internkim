export function saveFile(filename: string, content: string, contentType: string): void {
	const url = URL.createObjectURL(new Blob([content], { type: contentType }));
	const link = document.createElement('a');
	link.href = url;
	link.download = filename;
	link.click();
	setTimeout(() => URL.revokeObjectURL(url));
}

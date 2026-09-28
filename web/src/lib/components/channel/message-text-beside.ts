const imageMarkdownPattern = /!\[[^\]]*\]\((\S+?)\)/g;
const linkMarkdownPattern = /\[[^\]]*\]\((\S+?)\)/g;

// An imported message names each file it carries in its body, so a client that
// reads nothing but text still has them. This one draws them itself, and a
// reference to something already on screen is not text.
export function messageTextBeside(text: string, attachmentURLs: string[]): string {
	const withoutImages = text.replace(imageMarkdownPattern, '');
	const withoutFiles = withoutImages.replace(linkMarkdownPattern, (link, url: string) =>
		attachmentURLs.includes(url) ? '' : link
	);
	return withoutFiles.trim();
}

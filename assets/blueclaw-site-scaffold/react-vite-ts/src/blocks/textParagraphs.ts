export function splitParagraphs(body: string | undefined): string[] {
	if (!body) return [];
	return body.split(/\n{2,}/).filter((paragraph) => paragraph.trim().length > 0);
}

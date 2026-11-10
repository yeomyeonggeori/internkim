import { Fragment, Slice, type Node, type ResolvedPos, type Schema } from '@tiptap/pm/model';

const lineBreak = /\r\n?|\n/;

export function pastedTextSlice(text: string, context: ResolvedPos, schema: Schema): Slice {
	if (text === '') return Slice.empty;
	if (context.parent.type.spec.code) return new Slice(Fragment.from(schema.text(text)), 0, 0);
	const inline: Node[] = [];
	text.split(lineBreak).forEach((line, index) => {
		if (index > 0) inline.push(schema.nodes.hardBreak.create());
		if (line !== '') inline.push(schema.text(line));
	});
	return new Slice(Fragment.from(schema.nodes.paragraph.create(null, inline)), 1, 1);
}

export function pastedParagraphsAsCopiedLines(
	slice: Slice,
	plainText: string,
	context: ResolvedPos,
	schema: Schema
): Slice | undefined {
	if (context.parent.type.spec.code || plainText === '') return undefined;
	const paragraphs: Node[] = [];
	slice.content.forEach((block) => paragraphs.push(block));
	if (paragraphs.length < 2 || paragraphs.some((block) => block.type !== schema.nodes.paragraph)) return undefined;
	const breaks = breaksBetweenParagraphs(paragraphs, plainText.split(lineBreak));
	if (breaks === undefined) return pastedTextSlice(plainText, context, schema);
	const inline: Node[] = [];
	paragraphs.forEach((block, index) => {
		const count = index === 0 ? 0 : breaks[index - 1];
		for (let added = 0; added < count; added++) inline.push(schema.nodes.hardBreak.create());
		block.content.forEach((child) => inline.push(child));
	});
	return new Slice(Fragment.from(schema.nodes.paragraph.create(null, inline)), 1, 1);
}

function breaksBetweenParagraphs(paragraphs: Node[], lines: string[]): number[] | undefined {
	const breaks: number[] = [];
	let line = 0;
	for (const [index, paragraph] of paragraphs.entries()) {
		const text = comparable(paragraph.textContent);
		let skipped = 0;
		while (index > 0 && line < lines.length && comparable(lines[line]) === '' && text !== '') {
			skipped++;
			line++;
		}
		if (line >= lines.length || comparable(lines[line]) !== text) return undefined;
		if (index > 0) breaks.push(skipped + 1);
		line++;
	}
	return breaks;
}

function comparable(text: string): string {
	return text.replace(/\s+/g, ' ').trim();
}

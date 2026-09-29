export type ComposerFormat =
	| 'bold'
	| 'italic'
	| 'strikethrough'
	| 'link'
	| 'orderedList'
	| 'bulletList'
	| 'quote'
	| 'code';

export type ComposerDraft = {
	text: string;
	selectionStart: number;
	selectionEnd: number;
};

const inlineMarkers: Record<'bold' | 'italic' | 'strikethrough', string> = {
	bold: '**',
	italic: '*',
	strikethrough: '~~'
};

const linePrefixes: Record<'orderedList' | 'bulletList' | 'quote', (index: number) => string> = {
	bulletList: () => '- ',
	quote: () => '> ',
	orderedList: (index) => `${index + 1}. `
};

export function insertIntoDraft(draft: ComposerDraft, inserted: string): ComposerDraft {
	const cursor = draft.selectionStart + inserted.length;
	return {
		text: draft.text.slice(0, draft.selectionStart) + inserted + draft.text.slice(draft.selectionEnd),
		selectionStart: cursor,
		selectionEnd: cursor
	};
}

export function applyComposerFormat(draft: ComposerDraft, format: ComposerFormat): ComposerDraft {
	if (format === 'link') return wrapSelection(draft, '[', '](', ')', true);
	if (format === 'code') return formatCode(draft);
	if (format === 'orderedList' || format === 'bulletList' || format === 'quote') {
		return prefixLines(draft, linePrefixes[format]);
	}
	const marker = inlineMarkers[format];
	return wrapSelection(draft, marker, marker, '', false);
}

function formatCode(draft: ComposerDraft): ComposerDraft {
	const selected = draft.text.slice(draft.selectionStart, draft.selectionEnd);
	if (!selected.includes('\n')) return wrapSelection(draft, '`', '`', '', false);
	return wrapSelection(draft, '```\n', '\n```', '', false);
}

function wrapSelection(
	draft: ComposerDraft,
	opening: string,
	closing: string,
	trailing: string,
	placesCursorAfterClosing: boolean
): ComposerDraft {
	const selected = draft.text.slice(draft.selectionStart, draft.selectionEnd);
	const text =
		draft.text.slice(0, draft.selectionStart) +
		opening +
		selected +
		closing +
		trailing +
		draft.text.slice(draft.selectionEnd);
	const selectedStart = draft.selectionStart + opening.length;
	const selectedEnd = selectedStart + selected.length;
	if (placesCursorAfterClosing && selected.length > 0) {
		const cursor = selectedEnd + closing.length;
		return { text, selectionStart: cursor, selectionEnd: cursor };
	}
	return { text, selectionStart: selectedStart, selectionEnd: selectedEnd };
}

function prefixLines(draft: ComposerDraft, prefixFor: (index: number) => string): ComposerDraft {
	const blockStart = draft.text.lastIndexOf('\n', draft.selectionStart - 1) + 1;
	const nextBreak = draft.text.indexOf('\n', draft.selectionEnd);
	const blockEnd = nextBreak < 0 ? draft.text.length : nextBreak;
	const prefixed = draft.text
		.slice(blockStart, blockEnd)
		.split('\n')
		.map((line, index) => prefixFor(index) + line)
		.join('\n');
	const text = draft.text.slice(0, blockStart) + prefixed + draft.text.slice(blockEnd);
	if (draft.selectionStart === draft.selectionEnd) {
		const cursor = draft.selectionStart + prefixFor(0).length;
		return { text, selectionStart: cursor, selectionEnd: cursor };
	}
	return { text, selectionStart: blockStart, selectionEnd: blockStart + prefixed.length };
}

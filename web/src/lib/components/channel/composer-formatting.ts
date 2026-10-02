import type { Editor } from '@tiptap/core';

export type ComposerFormat =
	| 'bold'
	| 'italic'
	| 'strikethrough'
	| 'link'
	| 'orderedList'
	| 'bulletList'
	| 'quote'
	| 'code';

const formatNodeNames: Record<ComposerFormat, string> = {
	bold: 'bold',
	italic: 'italic',
	strikethrough: 'strike',
	link: 'link',
	orderedList: 'orderedList',
	bulletList: 'bulletList',
	quote: 'blockquote',
	code: 'code'
};

export function isComposerFormatActive(editor: Editor, format: ComposerFormat): boolean {
	if (format === 'code') return editor.isActive('code') || editor.isActive('codeBlock');
	return editor.isActive(formatNodeNames[format]);
}

export function applyComposerFormat(editor: Editor, format: ComposerFormat, askLinkAddress: () => string | null): void {
	const chain = editor.chain().focus();
	if (format === 'bold') chain.toggleBold().run();
	else if (format === 'italic') chain.toggleItalic().run();
	else if (format === 'strikethrough') chain.toggleStrike().run();
	else if (format === 'orderedList') chain.toggleOrderedList().run();
	else if (format === 'bulletList') chain.toggleBulletList().run();
	else if (format === 'quote') chain.toggleBlockquote().run();
	else if (format === 'code') toggleCode(editor);
	else toggleLink(editor, askLinkAddress);
}

function toggleCode(editor: Editor): void {
	const { $from, $to } = editor.state.selection;
	const spansBlocks = !$from.sameParent($to);
	if (spansBlocks || editor.isActive('codeBlock')) editor.chain().focus().toggleCodeBlock().run();
	else editor.chain().focus().toggleCode().run();
}

function toggleLink(editor: Editor, askLinkAddress: () => string | null): void {
	if (editor.isActive('link')) {
		editor.chain().focus().extendMarkRange('link').unsetLink().run();
		return;
	}
	const address = askLinkAddress()?.trim();
	if (!address) return;
	if (editor.state.selection.empty) {
		editor
			.chain()
			.focus()
			.insertContent({ type: 'text', text: address, marks: [{ type: 'link', attrs: { href: address } }] })
			.run();
		return;
	}
	editor.chain().focus().setLink({ href: address }).run();
}

import { Extension } from '@tiptap/core';
import type { Node } from '@tiptap/pm/model';
import { Plugin, PluginKey } from '@tiptap/pm/state';
import { Decoration, DecorationSet } from '@tiptap/pm/view';
import { mentionPieces, type MentionLabel } from '$lib/messenger/mention-text';

const mentionHighlightKey = new PluginKey<DecorationSet>('composer-mention-highlight');
const mentionAttributes = { class: 'rounded-sm bg-current/15 font-semibold', 'data-mention': '' };

function mentionDecorations(document: Node, labels: MentionLabel[]): DecorationSet {
	const decorations: Decoration[] = [];
	document.descendants((node, position) => {
		if (!node.isText || !node.text) return;
		let offset = position;
		for (const piece of mentionPieces(node.text, labels)) {
			if (piece.isMention) decorations.push(Decoration.inline(offset, offset + piece.text.length, mentionAttributes));
			offset += piece.text.length;
		}
	});
	return DecorationSet.create(document, decorations);
}

export function composerMentionHighlight(labels: () => MentionLabel[]): Extension {
	return Extension.create({
		name: 'composerMentionHighlight',
		addProseMirrorPlugins() {
			return [
				new Plugin<DecorationSet>({
					key: mentionHighlightKey,
					state: {
						init: (_config, state) => mentionDecorations(state.doc, labels()),
						apply: (transaction, previous, _oldState, state) =>
							transaction.docChanged ? mentionDecorations(state.doc, labels()) : previous
					},
					props: { decorations: (state) => mentionHighlightKey.getState(state) }
				})
			];
		}
	});
}

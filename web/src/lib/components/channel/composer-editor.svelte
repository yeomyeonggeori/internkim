<script lang="ts" module>
	export type TextBeforeCursor = { text: string; cursor: number };
</script>

<script lang="ts">
	import { Editor } from '@tiptap/core';
	import StarterKit from '@tiptap/starter-kit';
	import { MarkdownManager } from '@tiptap/markdown';
	import { onDestroy, onMount } from 'svelte';
	import {
		applyComposerFormat,
		isComposerFormatActive,
		type ComposerFormat
	} from './composer-formatting';
	import { composerDocumentOf, markdownOfComposer } from './composer-markdown';
	import type { WrittenMention } from '$lib/messenger/mention-draft';

	let {
		value = $bindable(''),
		activeFormats = $bindable([]),
		placeholder,
		disabled,
		ariaAttributes,
		onKeydown,
		onInput,
		onSelectionChange,
		onBlur
	}: {
		value?: string;
		activeFormats?: ComposerFormat[];
		placeholder: string;
		disabled: boolean;
		ariaAttributes: Record<string, string>;
		onKeydown: (event: KeyboardEvent) => boolean;
		onInput: () => void;
		onSelectionChange: () => void;
		onBlur: () => void;
	} = $props();

	const allFormats: ComposerFormat[] = [
		'bold',
		'italic',
		'strikethrough',
		'link',
		'orderedList',
		'bulletList',
		'quote',
		'code'
	];
	const hardBreakCharacter = '\n';
	const editorClass = 'markdown min-h-16 w-full px-2.5 py-2 text-base outline-none md:text-sm';

	const extensions = [StarterKit.configure({ heading: false, horizontalRule: false, underline: false })];
	const markdownParser = new MarkdownManager({ extensions });

	let element = $state<HTMLDivElement | null>(null);
	let editor = $state<Editor | null>(null);
	let emittedValue = '';

	export function focus(): void {
		editor?.commands.focus('end');
	}

	export function format(chosen: ComposerFormat, askLinkAddress: () => string | null): void {
		if (editor) applyComposerFormat(editor, chosen, askLinkAddress);
	}

	export function insertText(inserted: string): void {
		editor?.chain().focus().insertContent({ type: 'text', text: inserted }).run();
	}

	export function textBeforeCursor(): TextBeforeCursor | undefined {
		if (!editor) return undefined;
		const cursorPosition = editor.state.selection.$from;
		const text = cursorPosition.parent.textBetween(0, cursorPosition.parentOffset, undefined, hardBreakCharacter);
		return { text, cursor: text.length };
	}

	export function writeMention(written: WrittenMention): void {
		if (!editor) return;
		const blockStart = editor.state.selection.$from.start();
		editor
			.chain()
			.focus()
			.insertContentAt(
				{ from: blockStart + written.from, to: blockStart + written.to },
				{ type: 'text', text: written.inserted }
			)
			.run();
	}

	function splitWithoutSending(current: Editor): boolean {
		return current.commands.first(({ commands }) => [
			() => commands.newlineInCode(),
			() => commands.splitListItem('listItem'),
			() => commands.setHardBreak()
		]);
	}

	function handleKeyDown(event: KeyboardEvent): boolean {
		if (onKeydown(event)) return true;
		if (event.key === 'Enter' && event.shiftKey && !event.isComposing && editor) {
			return splitWithoutSending(editor);
		}
		return false;
	}

	function emit(current: Editor): void {
		emittedValue = current.isEmpty ? '' : markdownOfComposer(current.getJSON());
		value = emittedValue;
		onInput();
	}

	onMount(() => {
		if (!element) return;
		emittedValue = value;
		editor = new Editor({
			element,
			extensions,
			editable: !disabled,
			editorProps: {
				attributes: { ...ariaAttributes, class: editorClass, 'aria-multiline': 'true' },
				handleKeyDown: (_view, event) => handleKeyDown(event)
			},
			onUpdate: ({ editor: current }) => emit(current),
			onSelectionUpdate: () => onSelectionChange(),
			onTransaction: ({ editor: current }) => {
				activeFormats = allFormats.filter((candidate) => isComposerFormatActive(current, candidate));
			},
			onBlur: () => onBlur()
		});
		showValue(editor);
	});

	function showValue(current: Editor): void {
		const shown = composerDocumentOf(value, (markdown) => markdownParser.parse(markdown));
		current.commands.setContent(shown, { emitUpdate: false });
	}

	onDestroy(() => editor?.destroy());

	$effect(() => {
		if (!editor || value === emittedValue) return;
		emittedValue = value;
		showValue(editor);
	});

	$effect(() => {
		editor?.setEditable(!disabled);
	});

	$effect(() => {
		if (!editor) return;
		const attributes = { ...ariaAttributes, class: editorClass, 'aria-multiline': 'true' };
		editor.setOptions({ editorProps: { ...editor.options.editorProps, attributes } });
	});
</script>

<div class="relative w-full min-w-0 flex-1" data-slot="input-group-control">
	{#if value.trim() === ''}
		<span
			class="text-muted-foreground pointer-events-none absolute top-2 left-2.5 text-base md:text-sm"
			aria-hidden="true"
		>
			{placeholder}
		</span>
	{/if}
	<div bind:this={element}></div>
</div>

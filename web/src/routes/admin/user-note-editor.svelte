<!-- 사용자 메모 팝업 편집 UI를 제공한다. -->
<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Label } from '$lib/components/ui/label';
	import { Textarea } from '$lib/components/ui/textarea';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import type { NoteEditorPosition } from './user-note-position';

	type UserNoteEditorProps = {
		value: string;
		position: NoteEditorPosition;
		isSaving: boolean;
		noteLabel: string;
		placeholder: string;
		cancelLabel: string;
		saveLabel: string;
		onValueChange: (value: string) => void;
		onCancel: () => void;
		onSave: () => void;
	};

	let {
		value,
		position,
		isSaving,
		noteLabel,
		placeholder,
		cancelLabel,
		saveLabel,
		onValueChange,
		onCancel,
		onSave
	}: UserNoteEditorProps = $props();

	function handleKeydown(event: KeyboardEvent) {
		if (event.key !== 'Escape') return;
		event.stopPropagation();
		onCancel();
	}

	function handleInput(event: Event) {
		if (!(event.currentTarget instanceof HTMLTextAreaElement)) return;
		onValueChange(event.currentTarget.value);
	}
</script>

<div
	class="fixed z-50 w-[min(20rem,calc(100vw-2rem))] rounded-md border bg-popover p-3 text-popover-foreground shadow-lg"
	style={`left: ${position.left}px; top: ${position.top}px;`}
	role="dialog"
	aria-label={noteLabel}
	tabindex="-1"
	onkeydown={handleKeydown}
	onclick={(event) => event.stopPropagation()}
>
	<div class="grid gap-3">
		<label class="grid gap-1.5">
			<Label>{noteLabel}</Label>
			<Textarea value={value} oninput={handleInput} class="min-h-28 resize-y text-sm" placeholder={placeholder} autofocus />
		</label>
		<div class="flex justify-end gap-2">
			<Button type="button" variant="outline" size="sm" disabled={isSaving} onclick={onCancel}>{cancelLabel}</Button>
			<Button type="button" size="sm" disabled={isSaving} onclick={onSave}>
				{#if isSaving}
					<RefreshCwIcon class="size-4 animate-spin" />
				{/if}
				{saveLabel}
			</Button>
		</div>
	</div>
</div>

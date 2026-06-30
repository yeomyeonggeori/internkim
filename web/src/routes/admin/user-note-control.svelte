<!-- 사용자 메모 버튼과 팝업 편집 상태를 연결한다. -->
<script lang="ts">
	import type { AdminPageText } from './admin-types';
	import UserNoteEditor from './user-note-editor.svelte';
	import type { NoteEditorPosition } from './user-note-position';
	import UserNoteTrigger from './user-note-trigger.svelte';

	type UserNoteControlProps = {
		note: string | undefined;
		text: AdminPageText['users'];
		isOpen: boolean;
		draft: string;
		position: NoteEditorPosition;
		isSaving: boolean;
		onOpen: (event: MouseEvent) => void;
		onValueChange: (value: string) => void;
		onCancel: () => void;
		onSave: () => void;
	};

	let {
		note,
		text,
		isOpen,
		draft,
		position,
		isSaving,
		onOpen,
		onValueChange,
		onCancel,
		onSave
	}: UserNoteControlProps = $props();
</script>

<UserNoteTrigger note={note} {isOpen} label={text.noteEdit} fallbackTitle={text.noteEdit} {onOpen} />
{#if isOpen}
	<UserNoteEditor
		value={draft}
		{position}
		{isSaving}
		noteLabel={text.note}
		placeholder={text.notePlaceholder}
		cancelLabel={text.cancel}
		saveLabel={text.save}
		onValueChange={onValueChange}
		onCancel={onCancel}
		onSave={onSave}
	/>
{/if}

<!-- admin 사용자 디렉터리의 모바일 카드 행을 렌더링한다. -->
<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import type { UserRecordChanges } from './admin-user-record-changes';
	import type { AdminPageText, CircleRecord, UserRecord, UserRole } from './admin-types';
	import UserDirectoryActions from './user-directory-actions.svelte';
	import UserNoteControl from './user-note-control.svelte';
	import type { NoteEditorPosition } from './user-note-position';

	type UserDirectoryMobileCardProps = {
		record: UserRecord;
		visibleCircles: CircleRecord[];
		text: AdminPageText['users'];
		adminCount: number;
		isSaving: boolean;
		isValid: boolean;
		isNoteOpen: boolean;
		noteDraft: string;
		noteEditorPosition: NoteEditorPosition;
		hasUserCircle: (record: UserRecord, circleID: string) => boolean;
		onRecordChange: (record: UserRecord, changes: UserRecordChanges) => void;
		onToggleCircle: (record: UserRecord, circleID: string) => void;
		onOpenNote: (record: UserRecord, event: MouseEvent) => void;
		onNoteValueChange: (value: string) => void;
		onCancelNote: () => void;
		onSaveNote: (record: UserRecord) => void;
		onSave: (record: UserRecord, role?: UserRole) => void;
		onResetPassword: (record: UserRecord) => void;
		onRemove: (email: string) => void;
	};

	let {
		record,
		visibleCircles,
		text,
		adminCount,
		isSaving,
		isValid,
		isNoteOpen,
		noteDraft,
		noteEditorPosition,
		hasUserCircle,
		onRecordChange,
		onToggleCircle,
		onOpenNote,
		onNoteValueChange,
		onCancelNote,
		onSaveNote,
		onSave,
		onResetPassword,
		onRemove
	}: UserDirectoryMobileCardProps = $props();

</script>

<div class="relative grid gap-3 border-b p-4 last:border-b-0">
	<div class="flex min-w-0 items-start justify-between gap-3">
		<div class="flex min-w-0 items-center gap-3">
			<PersonAvatar name={record.name} email={record.email} class="size-10" />
			<div class="min-w-0">
				<p class="truncate text-sm font-medium">{record.name || record.email}</p>
				<p class="truncate text-xs text-muted-foreground">{record.email}</p>
				{#if record.mattermostUsername}
					<p class="truncate text-xs text-muted-foreground">Mattermost: {record.mattermostUsername}</p>
				{/if}
			</div>
		</div>
		<div class="flex shrink-0 items-center gap-2">
			<UserNoteControl
				note={record.note}
				{text}
				isOpen={isNoteOpen}
				draft={noteDraft}
				position={noteEditorPosition}
				isSaving={isSaving}
				onOpen={(event) => onOpenNote(record, event)}
				onValueChange={onNoteValueChange}
				onCancel={onCancelNote}
				onSave={() => onSaveNote(record)}
			/>
			<Badge variant={record.role === 'admin' ? 'secondary' : 'outline'}>{record.role}</Badge>
		</div>
	</div>
	<div class="grid gap-3 sm:grid-cols-3">
		<label class="grid gap-1.5">
			<Label>{text.handle}</Label>
			<Input value={record.handle} placeholder={text.handlePlaceholder} autocomplete="off" oninput={(event) => onRecordChange(record, { handle: event.currentTarget.value })} />
		</label>
		<label class="grid gap-1.5">
			<Label>{text.realName}</Label>
			<Input value={record.name} placeholder={text.realNamePlaceholder} autocomplete="off" oninput={(event) => onRecordChange(record, { name: event.currentTarget.value })} />
		</label>
		<label class="grid gap-1.5">
			<Label>{text.hireDate}</Label>
			<Input value={record.hireDate} type="date" oninput={(event) => onRecordChange(record, { hireDate: event.currentTarget.value })} />
		</label>
	</div>
	<div class="flex flex-wrap gap-1.5">
		{#each visibleCircles as circle (circle.circleID)}
			<Button
				type="button"
				variant={hasUserCircle(record, circle.circleID) ? 'secondary' : 'outline'}
				size="sm"
				disabled={circle.circleID === 'staff' || isSaving}
				onclick={() => onToggleCircle(record, circle.circleID)}
			>
				{circle.displayName || circle.circleID}
			</Button>
		{/each}
	</div>
	{#if record.isIncomplete}
		<p class="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">{text.incomplete}</p>
	{/if}
	<UserDirectoryActions
		{record}
		{text}
		{adminCount}
		{isSaving}
		{isValid}
		layout="mobile"
		{onSave}
		{onResetPassword}
		{onRemove}
	/>
</div>

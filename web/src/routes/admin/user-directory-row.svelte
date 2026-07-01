<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Table from '$lib/components/ui/table';
	import type { UserRecordChanges } from './admin-user-record-changes';
	import type { AdminPageText, CircleRecord, UserRecord, UserRole } from './admin-types';
	import UserDirectoryActions from './user-directory-actions.svelte';
	import UserNoteControl from './user-note-control.svelte';

	type UserDirectoryRowProps = {
		record: UserRecord;
		visibleCircles: CircleRecord[];
		text: AdminPageText['users'];
		adminCount: number;
		isSaving: boolean;
		isValid: boolean;
		hasUserCircle: (record: UserRecord, circleID: string) => boolean;
		onRecordChange: (record: UserRecord, changes: UserRecordChanges) => void;
		onToggleCircle: (record: UserRecord, circleID: string) => void;
		onSaveNote: (record: UserRecord, note: string) => Promise<boolean>;
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
		hasUserCircle,
		onRecordChange,
		onToggleCircle,
		onSaveNote,
		onSave,
		onResetPassword,
		onRemove
	}: UserDirectoryRowProps = $props();

</script>

<Table.Row>
	<Table.Cell>
		<div class="flex min-w-0 items-center gap-3">
			<PersonAvatar name={record.name} email={record.email} class="size-9" />
			<div class="relative min-w-0">
				<div class="flex min-w-0 items-center gap-1.5">
					<p class="truncate text-sm font-medium">{record.email}</p>
					<UserNoteControl note={record.note} {text} {isSaving} onSave={(note) => onSaveNote(record, note)} />
				</div>
				<p class="truncate text-xs text-muted-foreground">
					{record.mattermostUsername ? `Mattermost: ${record.mattermostUsername}` : text.noMattermost}
				</p>
				{#if record.isIncomplete}
					<p class="text-xs text-destructive">{text.incomplete}</p>
				{/if}
			</div>
		</div>
	</Table.Cell>
	<Table.Cell>
		<Input value={record.handle} placeholder={text.handlePlaceholder} autocomplete="off" oninput={(event) => onRecordChange(record, { handle: event.currentTarget.value })} />
	</Table.Cell>
	<Table.Cell>
		<Input value={record.name} placeholder={text.realNamePlaceholder} autocomplete="off" oninput={(event) => onRecordChange(record, { name: event.currentTarget.value })} />
	</Table.Cell>
	<Table.Cell>
		<Input value={record.hireDate} type="date" oninput={(event) => onRecordChange(record, { hireDate: event.currentTarget.value })} />
	</Table.Cell>
	<Table.Cell>
		<Badge variant={record.role === 'admin' ? 'secondary' : 'outline'}>{record.role}</Badge>
	</Table.Cell>
	<Table.Cell>
		<div class="flex flex-wrap gap-1.5">
			{#each visibleCircles as circle (circle.circleID)}
				<Button
					type="button"
					variant={hasUserCircle(record, circle.circleID) ? 'secondary' : 'outline'}
					size="sm"
					disabled={circle.circleID === 'staff' || isSaving}
					onclick={() => onToggleCircle(record, circle.circleID)}
					title={circle.isMattermostManaged ? text.mattermostManaged : ''}
				>
					{circle.displayName || circle.circleID}
				</Button>
			{/each}
		</div>
	</Table.Cell>
	<Table.Cell>
		<UserDirectoryActions
			{record}
			{text}
			{adminCount}
			{isSaving}
			{isValid}
			layout="desktop"
			{onSave}
			{onResetPassword}
			{onRemove}
		/>
	</Table.Cell>
</Table.Row>

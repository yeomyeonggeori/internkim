<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import type { UserRecordChanges } from './admin-user-record-changes';
	import type { AdminPageText, CircleRecord, UserRecord, UserRole } from './admin-types';
	import UserDirectoryMobileCard from './user-directory-mobile-card.svelte';
	import UserDirectoryRow from './user-directory-row.svelte';
	import { noteEditorPositionFromButton, type NoteEditorPosition } from './user-note-position';

	type UserDirectoryPanelProps = {
		records: UserRecord[];
		visibleCircles: CircleRecord[];
		text: AdminPageText['users'];
		adminCount: number;
		isLoading: boolean;
		isSaving: boolean;
		hasUserCircle: (record: UserRecord, circleID: string) => boolean;
		isValidUserRecord: (record: UserRecord) => boolean;
		onRecordChange: (record: UserRecord, changes: UserRecordChanges) => void;
		onToggleCircle: (record: UserRecord, circleID: string) => void;
		onSaveNote: (record: UserRecord, note: string) => Promise<boolean>;
		onSave: (record: UserRecord, role?: UserRole) => Promise<boolean>;
		onResetPassword: (record: UserRecord) => void;
		onRemove: (email: string) => void;
	};

	let {
		records,
		visibleCircles,
		text,
		adminCount,
		isLoading,
		isSaving,
		hasUserCircle,
		isValidUserRecord,
		onRecordChange,
		onToggleCircle,
		onSaveNote,
		onSave,
		onResetPassword,
		onRemove
	}: UserDirectoryPanelProps = $props();

	let openNoteEmail = $state('');
	let noteDraft = $state('');
	let noteEditorPosition = $state<NoteEditorPosition>({ left: 16, top: 16 });

	function isNoteEditorOpen(record: UserRecord): boolean {
		return openNoteEmail === record.email;
	}

	function openNoteEditor(record: UserRecord, event: MouseEvent): void {
		openNoteEmail = record.email;
		noteDraft = record.note ?? '';
		noteEditorPosition = noteEditorPositionFromButton(event.currentTarget);
	}

	function cancelNoteEditor(): void {
		openNoteEmail = '';
		noteDraft = '';
	}

	async function saveNoteDraft(record: UserRecord): Promise<void> {
		const didSave = await onSaveNote(record, noteDraft);
		if (didSave) cancelNoteEditor();
	}
</script>

{#if isLoading}
	<p class="text-muted-foreground text-sm">{text.loading}</p>
{:else if records.length === 0}
	<p class="text-muted-foreground text-sm">{text.empty}</p>
{:else}
	<Card.Root class="overflow-visible">
		<Card.Header class="flex-row items-center justify-between gap-3 border-b">
			<div>
				<Card.Title class="text-sm">{text.directoryTitle}</Card.Title>
				<Card.Description>{text.directoryDescription}</Card.Description>
			</div>
			<Badge variant="secondary">{adminCount} {text.adminCount}</Badge>
		</Card.Header>
		<Card.Content class="p-0">
			<div class="hidden overflow-x-auto overflow-y-visible lg:block">
				<Table.Root class="min-w-[1260px]">
					<Table.Header class="bg-muted/40">
						<Table.Row class="hover:bg-transparent">
							<Table.Head class="w-[250px]">{text.person}</Table.Head>
							<Table.Head class="w-[160px]">{text.handle}</Table.Head>
							<Table.Head class="w-[180px]">{text.realName}</Table.Head>
							<Table.Head class="w-[150px]">{text.hireDate}</Table.Head>
							<Table.Head class="w-[110px]">{text.role}</Table.Head>
							<Table.Head class="w-[220px]">{text.groups}</Table.Head>
							<Table.Head class="text-right">{text.actions}</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each records as record (record.email)}
							<UserDirectoryRow
								{record}
								{visibleCircles}
								{text}
								{adminCount}
								{hasUserCircle}
								isSaving={isSaving}
								isValid={isValidUserRecord(record)}
								isNoteOpen={isNoteEditorOpen(record)}
								{noteDraft}
								{noteEditorPosition}
								onRecordChange={onRecordChange}
								onToggleCircle={onToggleCircle}
								onOpenNote={openNoteEditor}
								onNoteValueChange={(value) => (noteDraft = value)}
								onCancelNote={cancelNoteEditor}
								onSaveNote={saveNoteDraft}
								{onSave}
								{onResetPassword}
								{onRemove}
							/>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
			<div class="grid gap-0 lg:hidden">
				{#each records as record (record.email)}
					<UserDirectoryMobileCard
						{record}
						{visibleCircles}
						{text}
						{adminCount}
						{hasUserCircle}
						isSaving={isSaving}
						isValid={isValidUserRecord(record)}
						isNoteOpen={isNoteEditorOpen(record)}
						{noteDraft}
						{noteEditorPosition}
						onRecordChange={onRecordChange}
						onToggleCircle={onToggleCircle}
						onOpenNote={openNoteEditor}
						onNoteValueChange={(value) => (noteDraft = value)}
						onCancelNote={cancelNoteEditor}
						onSaveNote={saveNoteDraft}
						{onSave}
						{onResetPassword}
						{onRemove}
					/>
				{/each}
			</div>
		</Card.Content>
	</Card.Root>
{/if}

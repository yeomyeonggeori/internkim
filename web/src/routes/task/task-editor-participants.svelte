<script lang="ts">
	import CheckIcon from '@lucide/svelte/icons/check';
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Command from '$lib/components/ui/command';
	import * as Popover from '$lib/components/ui/popover';
	import XIcon from '@lucide/svelte/icons/x';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { toggleTaskParticipantID } from './task-draft';
	import type { TaskEditorText } from './task-editor-types';
	import type { TaskMember, Task } from './task-types';

	type Props = {
		taskDraft: Task;
		members: TaskMember[];
		canEditTask: boolean;
		canEditTaskAssignment: boolean;
		text: TaskEditorText;
		setParticipantIDs: (memberIDs: string[]) => void;
		removeParticipantID: (memberID: string) => void;
		canRemoveParticipant: (task: Task, memberID: string) => boolean;
	};

	let {
		taskDraft,
		members,
		canEditTask,
		canEditTaskAssignment,
		text,
		setParticipantIDs,
		removeParticipantID,
		canRemoveParticipant
	}: Props = $props();

	let isPickerOpen = $state(false);
	let selectedParticipantIDs = $derived(new Set(taskDraft.participantIDs));

	function participant(memberID: string): TaskMember | undefined {
		return members.find((member) => member.id === memberID);
	}

	function participantName(memberID: string, index: number): string {
		return participant(memberID)?.name ?? taskDraft.participantNames[index] ?? '';
	}

	function toggleParticipant(memberID: string): void {
		if (!canEditTask || !canEditTaskAssignment) return;
		const nextParticipantIDs = toggleTaskParticipantID(
			taskDraft,
			memberID,
			canRemoveParticipant(taskDraft, memberID)
		);
		setParticipantIDs(nextParticipantIDs);
	}
</script>

<div class="space-y-2">
	<div class="text-xs font-medium text-muted-foreground">{text.participants}</div>
	<Popover.Root bind:open={isPickerOpen}>
		<Popover.Trigger disabled={!canEditTask || !canEditTaskAssignment}>
			{#snippet child({ props })}
				<Button
					{...props}
					variant="outline"
					role="combobox"
					aria-label={text.participants}
					aria-expanded={isPickerOpen}
					disabled={!canEditTask || !canEditTaskAssignment}
					class="w-full justify-between font-normal"
				>
					<span class="truncate text-muted-foreground">{text.participantsPlaceholder}</span>
					<ChevronsUpDownIcon class="size-4 shrink-0 opacity-50" />
				</Button>
			{/snippet}
		</Popover.Trigger>
		<Popover.Content class="z-[52] w-[var(--bits-popover-anchor-width)] p-0" align="start" side="top">
			<Command.Root>
				<Command.Input placeholder={text.participantsPlaceholder} />
				<Command.List>
					<Command.Empty>{text.participantsPlaceholder}</Command.Empty>
					<Command.Group value="participants">
						{#each members as member (member.id)}
							<Command.Item
								value={member.id}
								keywords={[member.name, displayPersonName(member.name), member.email]}
								data-checked={selectedParticipantIDs.has(member.id)}
								onSelect={() => toggleParticipant(member.id)}
							>
								<PersonAvatar name={displayPersonName(member.name)} email={member.email} seed={member.id} image={member.image ?? ''} class="size-5" />
								<span class="min-w-0 flex-1">
									<span class="block truncate">{displayPersonName(member.name)}</span>
									<span class="block truncate text-xs text-muted-foreground">{member.email}</span>
								</span>
								{#if selectedParticipantIDs.has(member.id)}
									<CheckIcon class="size-4 shrink-0" />
								{/if}
							</Command.Item>
						{/each}
					</Command.Group>
				</Command.List>
			</Command.Root>
		</Popover.Content>
	</Popover.Root>
	<div class="flex flex-wrap gap-1">
		{#each taskDraft.participantIDs as participantID, index (participantID)}
			{@const selectedParticipant = participant(participantID)}
			{@const name = participantName(participantID, index)}
			{@const email = selectedParticipant?.email ?? ''}
			<Badge variant="outline" class="gap-1.5 pl-1 pr-1">
				<PersonAvatar name={displayPersonName(name)} {email} seed={participantID || name} image={selectedParticipant?.image ?? ''} class="size-4" />
				{displayPersonName(name)}
				{#if email}
					<span class="text-[10px] text-muted-foreground">{email}</span>
				{/if}
				{#if canEditTask && canEditTaskAssignment && canRemoveParticipant(taskDraft, participantID)}
					<button
						type="button"
						class="-mr-0.5 inline-flex size-4 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground"
						aria-label={text.removeParticipantAction.replace('{name}', email ? `${name} (${email})` : name)}
						onclick={() => removeParticipantID(participantID)}
					>
						<XIcon class="size-3" />
					</button>
				{/if}
			</Badge>
		{/each}
	</div>
</div>

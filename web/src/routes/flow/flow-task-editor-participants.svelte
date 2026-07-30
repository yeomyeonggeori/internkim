<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { TagsInput } from '$lib/components/ui/tags-input';
	import XIcon from '@lucide/svelte/icons/x';
	import { canRemoveFlowTaskParticipant } from './flow-task-workspace-model';
	import type { FlowTaskEditorText } from './flow-task-editor-types';
	import type { FlowMember, FlowTask } from './flow-types';

	type Props = {
		taskDraft: FlowTask;
		members: FlowMember[];
		canEditTask: boolean;
		canEditTaskAssignment: boolean;
		text: FlowTaskEditorText;
		setParticipantNames: (names: string[]) => void;
		removeParticipantID: (memberID: string) => void;
	};

	let {
		taskDraft,
		members,
		canEditTask,
		canEditTaskAssignment,
		text,
		setParticipantNames,
		removeParticipantID
	}: Props = $props();

	function participantImage(memberID: string): string {
		return members.find((member) => member.id === memberID)?.image ?? '';
	}
</script>

<div class="space-y-2">
	<div class="text-xs font-medium text-muted-foreground">{text.participants}</div>
	<TagsInput
		value={taskDraft.participantNames}
		suggestions={members.map((member) => member.name)}
		restrictToSuggestions
		showSelectedTags={false}
		suggestionsPlacement="top"
		placeholder={text.participantsPlaceholder}
		disabled={!canEditTask || !canEditTaskAssignment}
		onValueChange={setParticipantNames}
	/>
	<div class="flex flex-wrap gap-1">
		{#each taskDraft.participantNames as name, index}
			{@const participantID = taskDraft.participantIDs[index] ?? ''}
			<Badge variant="outline" class="gap-1.5 pl-1 pr-1">
				<PersonAvatar name={name} email={members.find((member) => member.id === participantID)?.email ?? ''} seed={participantID || name} image={participantImage(participantID)} class="size-4" />
				{name}
				{#if canEditTask && canEditTaskAssignment && canRemoveFlowTaskParticipant(taskDraft, participantID)}
					<button
						type="button"
						class="-mr-0.5 inline-flex size-4 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground"
						aria-label={text.removeParticipantAction.replace('{name}', name)}
						onclick={() => removeParticipantID(participantID)}
					>
						<XIcon class="size-3" />
					</button>
				{/if}
			</Badge>
		{/each}
	</div>
</div>

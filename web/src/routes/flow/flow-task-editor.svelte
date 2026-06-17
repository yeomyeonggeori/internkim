<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { Input } from '$lib/components/ui/input';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Select from '$lib/components/ui/select';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
	import { TagsInput } from '$lib/components/ui/tags-input';
	import XIcon from '@lucide/svelte/icons/x';
	import { tick } from 'svelte';
	import { isFlowStatusRejected, isFlowStatusRequested, isFlowStatusStopped } from './flow-status';
	import { sizeBadgeClass, statusBadgeClass } from './flow-style';
	import { canRemoveFlowTaskParticipant, flowBusinessLabel, flowBusinessOptionValue, flowBusinessValueFromOption } from './flow-task-workspace-model';
	import type { FlowMember, FlowTask } from './flow-types';

	type Option = {
		value: string;
		label: string;
	};

	type TaskText = {
		editTitle: string;
		createTitle: string;
		content: string;
		contentPlaceholder: string;
		goal: string;
		goalPlaceholder: string;
		owner: string;
		status: string;
		business: string;
		businessFallback: string;
		type: string;
		size: string;
		flag: string;
		startDate: string;
		endDate: string;
		participants: string;
		participantsPlaceholder: string;
		removeParticipantAction: string;
		requestReason: string;
		reason: string;
		dateRule: string;
		readOnly: string;
		deleteAction: string;
		deleteTitle: string;
		deleteDescription: string;
		deleteConfirm: string;
		cancel: string;
		deleteError: string;
		saving: string;
		deleting: string;
		save: string;
	};

	type Props = {
		taskDraft: FlowTask | null;
		members: FlowMember[];
		memberOptions: Option[];
		categoryOptions: Option[];
		typeOptions: Option[];
		sizeOptions: Option[];
		statusOptions: Option[];
		taskErrorMessage: string;
		isSavingTask: boolean;
		isDeletingTask: boolean;
		pageTitle: string;
		text: TaskText;
		statusLabel: (status: string) => string;
		setTaskOwnerID: (memberID: string) => void;
		setParticipantNames: (names: string[]) => void;
		removeParticipantID: (memberID: string) => void;
		saveTask: () => void;
		deleteTask: (task: FlowTask) => Promise<void>;
		canUpdateTask: (task: FlowTask) => boolean;
		canDeleteTask: (task: FlowTask) => boolean;
		canManageTaskAssignment: (task: FlowTask) => boolean;
		closeEditor: () => void;
	};

	let {
		taskDraft = $bindable<FlowTask | null>(null),
		members,
		memberOptions,
		categoryOptions,
		typeOptions,
		sizeOptions,
		statusOptions,
		taskErrorMessage,
		isSavingTask,
		isDeletingTask,
		pageTitle,
		text,
		statusLabel,
		setTaskOwnerID,
		setParticipantNames,
		removeParticipantID,
		saveTask,
		deleteTask,
		canUpdateTask,
		canDeleteTask,
		canManageTaskAssignment,
		closeEditor
	}: Props = $props();

	let canEditTask = $derived(taskDraft ? canUpdateTask(taskDraft) : false);
	let canRemoveTask = $derived(taskDraft ? Boolean(taskDraft.id) && canDeleteTask(taskDraft) : false);
	let canEditTaskAssignment = $derived(taskDraft ? canManageTaskAssignment(taskDraft) : false);
	let businessSelectValue = $derived(taskDraft ? flowBusinessOptionValue(taskDraft.business) : '');

	function memberOptionLabel(memberID: string): string {
		return memberOptions.find((option) => option.value === memberID)?.label ?? '-';
	}

	function memberOptionEmail(memberID: string): string {
		return members.find((member) => member.id === memberID)?.email ?? '';
	}

	function updateBusiness(value: string): void {
		if (!taskDraft) return;
		taskDraft.business = flowBusinessValueFromOption(value);
	}

	async function confirmTaskDelete(task: FlowTask): Promise<void> {
		closeEditor();
		await tick();
		confirmDelete({
			title: text.deleteTitle,
			description: text.deleteDescription.replace('{task}', task.content),
			confirm: { text: text.deleteConfirm },
			cancel: { text: text.cancel },
			onConfirm: async () => deleteTask(task)
		});
	}
</script>

<Sheet.Root open={taskDraft !== null} onOpenChange={(open) => {
	if (!open) closeEditor();
}}>
	<Sheet.Content class="w-full overflow-y-auto sm:max-w-xl">
		<Sheet.Header>
			<Sheet.Title>{taskDraft?.id ? text.editTitle : text.createTitle}</Sheet.Title>
			<Sheet.Description>{taskDraft?.weekCode} · {pageTitle}</Sheet.Description>
		</Sheet.Header>
		{#if taskDraft}
			<div class="space-y-4 px-4 pb-6">
				{#if !canEditTask}
					<div class="rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">
						{text.readOnly}
					</div>
				{/if}
				<div class="flex flex-wrap gap-2">
					<Badge class={statusBadgeClass(taskDraft.status)}>{statusLabel(taskDraft.status)}</Badge>
					<Badge class={sizeBadgeClass(taskDraft.size)}>{taskDraft.size}</Badge>
					<Badge variant="outline">{flowBusinessLabel(taskDraft.business, text.businessFallback)}</Badge>
					<Badge variant="outline">{taskDraft.type}</Badge>
				</div>
				<label class="grid gap-1 text-xs font-medium text-muted-foreground">
					{text.content}
					<Input bind:value={taskDraft.content} placeholder={text.contentPlaceholder} disabled={!canEditTask} />
				</label>
				<label class="grid gap-1 text-xs font-medium text-muted-foreground">
					{text.goal}
					<Input bind:value={taskDraft.goal} placeholder={text.goalPlaceholder} disabled={!canEditTask} />
				</label>
				<div class="grid gap-3 md:grid-cols-2">
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.owner}
						<Select.Root type="single" value={taskDraft.ownerID} onValueChange={setTaskOwnerID} disabled={!canEditTask || !canEditTaskAssignment}>
							<Select.Trigger class="w-full">
								<span class="flex min-w-0 items-center gap-2">
									<PersonAvatar name={memberOptionLabel(taskDraft.ownerID)} email={memberOptionEmail(taskDraft.ownerID)} seed={taskDraft.ownerID} class="size-5" />
									<span class="truncate">{memberOptionLabel(taskDraft.ownerID)}</span>
								</span>
							</Select.Trigger>
							<Select.Content>
								{#each memberOptions as option (option.value)}
									<Select.Item value={option.value} label={option.label}>
										<span class="flex min-w-0 items-center gap-2">
											<PersonAvatar name={option.label} email={memberOptionEmail(option.value)} seed={option.value} class="size-5" />
											<span class="truncate">{option.label}</span>
										</span>
									</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</label>
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.status}
						<Select.Root type="single" bind:value={taskDraft.status} disabled={!canEditTask}>
							<Select.Trigger class="w-full">
								{statusLabel(taskDraft.status)}
							</Select.Trigger>
							<Select.Content>
								{#each statusOptions as option (option.value)}
									<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</label>
					{#if categoryOptions.length > 1}
						<label class="grid gap-1 text-xs font-medium text-muted-foreground">
							{text.business}
							<Select.Root type="single" value={businessSelectValue} onValueChange={updateBusiness} disabled={!canEditTask}>
								<Select.Trigger class="w-full">
									{flowBusinessLabel(taskDraft.business, text.businessFallback)}
								</Select.Trigger>
								<Select.Content>
									{#each categoryOptions as option (option.value)}
										<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</label>
					{/if}
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.type}
						<Select.Root type="single" bind:value={taskDraft.type} disabled={!canEditTask}>
							<Select.Trigger class="w-full">
								{taskDraft.type || '-'}
							</Select.Trigger>
							<Select.Content>
								{#each typeOptions as option (option.value)}
									<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</label>
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.size}
						<Select.Root type="single" bind:value={taskDraft.size} disabled={!canEditTask}>
							<Select.Trigger class="w-full">
								{sizeOptions.find((option) => option.value === taskDraft?.size)?.label ?? '-'}
							</Select.Trigger>
							<Select.Content>
								{#each sizeOptions as option (option.value)}
									<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</label>
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.flag}
						<Input type="number" min={0} step={1} bind:value={taskDraft.flag} disabled={!canEditTask} />
					</label>
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.startDate}
						<Input type="date" bind:value={taskDraft.startDate} disabled={!canEditTask} />
					</label>
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.endDate}
						<Input type="date" bind:value={taskDraft.endDate} disabled={!canEditTask} />
					</label>
				</div>
				<div class="space-y-2">
					<div class="text-xs font-medium text-muted-foreground">{text.participants}</div>
					<TagsInput
						value={taskDraft.participantNames}
						suggestions={members.map((member) => member.name)}
						restrictToSuggestions
						showSelectedTags={false}
						placeholder={text.participantsPlaceholder}
						disabled={!canEditTask || !canEditTaskAssignment}
						onValueChange={setParticipantNames}
					/>
					<div class="flex flex-wrap gap-1">
						{#each taskDraft.participantNames as name, index}
							<Badge variant="outline" class="gap-1.5 pl-1 pr-1">
								<PersonAvatar name={name} seed={taskDraft.participantIDs[index] ?? name} class="size-4" />
								{name}
								{#if canEditTask && canEditTaskAssignment && canRemoveFlowTaskParticipant(taskDraft, taskDraft.participantIDs[index] ?? '')}
									<button
										type="button"
										class="-mr-0.5 inline-flex size-4 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground"
										aria-label={text.removeParticipantAction.replace('{name}', name)}
										onclick={() => removeParticipantID(taskDraft?.participantIDs[index] ?? '')}
									>
										<XIcon class="size-3" />
									</button>
								{/if}
							</Badge>
						{/each}
					</div>
				</div>
				{#if isFlowStatusRequested(taskDraft.status)}
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.requestReason}
						<Input bind:value={taskDraft.requestReason} disabled={!canEditTask} />
					</label>
				{/if}
				{#if isFlowStatusRejected(taskDraft.status) || isFlowStatusStopped(taskDraft.status)}
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.reason}
						<Input bind:value={taskDraft.decisionReason} disabled={!canEditTask} />
					</label>
				{/if}
				<Separator />
				<div class="rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">
					{text.dateRule}
				</div>
				{#if taskErrorMessage}
					<p class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{taskErrorMessage}</p>
				{/if}
				<Sheet.Footer>
					{#if canRemoveTask}
						<Button variant="destructive" onclick={() => confirmTaskDelete(taskDraft)} disabled={isDeletingTask || isSavingTask}>
							{isDeletingTask ? text.deleting : text.deleteAction}
						</Button>
					{/if}
					{#if canEditTask}
						<Button onclick={saveTask} disabled={isSavingTask || !taskDraft.content.trim()}>
							{isSavingTask ? text.saving : text.save}
						</Button>
					{/if}
				</Sheet.Footer>
			</div>
		{/if}
	</Sheet.Content>
</Sheet.Root>

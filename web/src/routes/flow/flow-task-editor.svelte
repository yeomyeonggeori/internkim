<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
	import { TagsInput } from '$lib/components/ui/tags-input';
	import { isFlowStatusRejected, isFlowStatusRequested, isFlowStatusStopped } from './flow-status';
	import { sizeBadgeClass, statusBadgeClass } from './flow-style';
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
		type: string;
		size: string;
		flag: string;
		startDate: string;
		endDate: string;
		participants: string;
		participantsPlaceholder: string;
		requestReason: string;
		reason: string;
		dateRule: string;
		saving: string;
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
		pageTitle: string;
		text: TaskText;
		statusLabel: (status: string) => string;
		setParticipantNames: (names: string[]) => void;
		saveTask: () => void;
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
		pageTitle,
		text,
		statusLabel,
		setParticipantNames,
		saveTask,
		closeEditor
	}: Props = $props();
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
				<div class="flex flex-wrap gap-2">
					<Badge class={statusBadgeClass(taskDraft.status)}>{statusLabel(taskDraft.status)}</Badge>
					<Badge class={sizeBadgeClass(taskDraft.size)}>{taskDraft.size}</Badge>
					{#if taskDraft.business}
						<Badge variant="outline">{taskDraft.business}</Badge>
					{/if}
					<Badge variant="outline">{taskDraft.type}</Badge>
				</div>
				<label class="grid gap-1 text-xs font-medium text-muted-foreground">
					{text.content}
					<Input bind:value={taskDraft.content} placeholder={text.contentPlaceholder} />
				</label>
				<label class="grid gap-1 text-xs font-medium text-muted-foreground">
					{text.goal}
					<Input bind:value={taskDraft.goal} placeholder={text.goalPlaceholder} />
				</label>
				<div class="grid gap-3 md:grid-cols-2">
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.owner}
						<Select.Root type="single" bind:value={taskDraft.ownerID}>
							<Select.Trigger class="w-full">
								{memberOptions.find((option) => option.value === taskDraft?.ownerID)?.label ?? '-'}
							</Select.Trigger>
							<Select.Content>
								{#each memberOptions as option (option.value)}
									<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</label>
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.status}
						<Select.Root type="single" bind:value={taskDraft.status}>
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
							<Select.Root type="single" bind:value={taskDraft.business}>
								<Select.Trigger class="w-full">
									{taskDraft.business || '-'}
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
						<Select.Root type="single" bind:value={taskDraft.type}>
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
						<Select.Root type="single" bind:value={taskDraft.size}>
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
						<Input type="number" min={0} step={1} bind:value={taskDraft.flag} />
					</label>
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.startDate}
						<Input type="date" bind:value={taskDraft.startDate} />
					</label>
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.endDate}
						<Input type="date" bind:value={taskDraft.endDate} />
					</label>
				</div>
				<div class="space-y-2">
					<div class="text-xs font-medium text-muted-foreground">{text.participants}</div>
					<TagsInput
						value={taskDraft.participantNames}
						suggestions={members.map((member) => member.name)}
						restrictToSuggestions
						placeholder={text.participantsPlaceholder}
						onValueChange={setParticipantNames}
					/>
				</div>
				{#if isFlowStatusRequested(taskDraft.status)}
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.requestReason}
						<Input bind:value={taskDraft.requestReason} />
					</label>
				{/if}
				{#if isFlowStatusRejected(taskDraft.status) || isFlowStatusStopped(taskDraft.status)}
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.reason}
						<Input bind:value={taskDraft.decisionReason} />
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
					<Button onclick={saveTask} disabled={isSavingTask || !taskDraft.content.trim()}>
						{isSavingTask ? text.saving : text.save}
					</Button>
				</Sheet.Footer>
			</div>
		{/if}
	</Sheet.Content>
</Sheet.Root>

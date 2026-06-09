<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Textarea } from '$lib/components/ui/textarea';
	import SparklesIcon from '@lucide/svelte/icons/sparkles';
	import { flowText } from './text';

	type FlowPageText = typeof flowText.ko;

	type Props = {
		quickTaskText: string;
		taskErrorMessage: string;
		quickTaskDuplicateMessage: string;
		isCreatingQuickTask: boolean;
		hasMembers: boolean;
		text: FlowPageText['task'];
		createQuickTask: () => Promise<void>;
		confirmQuickTaskDuplicate: () => Promise<void>;
	};

	let {
		quickTaskText = $bindable(''),
		taskErrorMessage,
		quickTaskDuplicateMessage,
		isCreatingQuickTask,
		hasMembers,
		text,
		createQuickTask,
		confirmQuickTaskDuplicate
	}: Props = $props();
</script>

<Card.Root size="sm">
	<Card.Content class="space-y-3">
		<div class="grid gap-2 md:grid-cols-[1fr_auto] md:items-stretch">
			<div>
				<Textarea
					class="min-h-16 resize-none"
					placeholder={text.quickAddPlaceholder}
					bind:value={quickTaskText}
				/>
				<p class="mt-1 text-xs text-muted-foreground">{text.quickAddHint}</p>
			</div>
			<Button
				class="md:h-auto md:min-h-16 md:px-5"
				onclick={createQuickTask}
				disabled={isCreatingQuickTask || !quickTaskText.trim() || !hasMembers}
			>
				<SparklesIcon />
				{isCreatingQuickTask ? text.saving : text.quickAdd}
			</Button>
		</div>
		{#if taskErrorMessage}
			<p class="rounded-lg border border-destructive/30 bg-destructive/10 p-2 text-sm text-destructive">{taskErrorMessage}</p>
		{/if}
		{#if quickTaskDuplicateMessage}
			<div class="flex flex-col gap-2 rounded-lg border border-amber-200 bg-amber-50 p-2 text-sm text-amber-900 md:flex-row md:items-center md:justify-between">
				<p>{quickTaskDuplicateMessage}</p>
				<Button variant="outline" size="sm" onclick={confirmQuickTaskDuplicate} disabled={isCreatingQuickTask}>
					{text.quickAddDuplicateAction}
				</Button>
			</div>
		{/if}
	</Card.Content>
</Card.Root>

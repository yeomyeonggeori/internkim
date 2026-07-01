<script lang="ts">
	import { Button } from '$lib/components/ui/button';
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
		textareaElement: HTMLTextAreaElement | null;
		submitButtonElement: HTMLElement | null;
		submitQuickTask: () => Promise<void>;
		confirmQuickTaskDuplicate: () => Promise<void>;
	};

	let {
		quickTaskText = $bindable(''),
		taskErrorMessage,
		quickTaskDuplicateMessage,
		isCreatingQuickTask,
		hasMembers,
		text,
		textareaElement = $bindable(null),
		submitButtonElement = $bindable(null),
		submitQuickTask,
		confirmQuickTaskDuplicate
	}: Props = $props();
</script>

<header class="space-y-1 px-5 pt-5">
	<h2 id="flow-ai-quick-add-title" class="font-semibold leading-none">{text.quickAdd}</h2>
	<p id="flow-ai-quick-add-description" class="text-sm text-muted-foreground">{text.quickAddHint}</p>
</header>
<div class="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-5 py-4">
	<Textarea
		bind:ref={textareaElement}
		class="min-h-0 flex-1 resize-none"
		placeholder={text.quickAddPlaceholder}
		bind:value={quickTaskText}
	/>
	<Button
		bind:ref={submitButtonElement}
		class="w-full"
		onclick={submitQuickTask}
		disabled={isCreatingQuickTask || !quickTaskText.trim() || !hasMembers}
	>
		<SparklesIcon />
		{isCreatingQuickTask ? text.saving : text.quickAdd}
	</Button>
	{#if taskErrorMessage}
		<p class="rounded-lg border border-destructive/30 bg-destructive/10 p-2 text-sm text-destructive">{taskErrorMessage}</p>
	{/if}
	{#if quickTaskDuplicateMessage}
		<div class="flex flex-col gap-2 rounded-lg border border-amber-200 bg-amber-50 p-2 text-sm text-amber-900">
			<p>{quickTaskDuplicateMessage}</p>
			<Button
				variant="outline"
				size="sm"
				onclick={confirmQuickTaskDuplicate}
				disabled={isCreatingQuickTask}
			>
				{text.quickAddDuplicateAction}
			</Button>
		</div>
	{/if}
</div>

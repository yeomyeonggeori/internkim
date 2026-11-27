<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import LeaveRequestForm from './leave-request-form.svelte';
	import type { LeaveRequestDraft } from './leave-request-draft.svelte';

	type Props = {
		title: string;
		description: string;
		closeLabel: string;
		draft: LeaveRequestDraft;
		onClose: () => void;
		testID: string;
	};

	let { title, description, closeLabel, draft, onClose, testID }: Props = $props();
</script>

<Dialog.Content
	class="top-0 left-0 h-svh max-h-none max-w-none translate-x-0 translate-y-0 grid-rows-[auto_minmax(0,1fr)] gap-0 overflow-hidden rounded-none p-0 sm:top-1/2 sm:left-1/2 sm:h-[min(90vh,56rem)] sm:max-w-3xl sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-xl"
	{closeLabel}
	data-testid={testID}
	data-mobile-fullscreen
>
	<div class="border-b bg-popover px-4 py-5 sm:px-6">
		<Dialog.Header class="pr-10">
			<Dialog.Title class="text-xl">{title}</Dialog.Title>
			<Dialog.Description>{description}</Dialog.Description>
		</Dialog.Header>
	</div>
	<div class="min-h-0 overflow-y-auto">
		<LeaveRequestForm {draft} onSubmitted={onClose} {onClose} />
	</div>
</Dialog.Content>

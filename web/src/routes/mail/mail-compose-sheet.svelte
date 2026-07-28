<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Textarea } from '$lib/components/ui/textarea';
	import SendIcon from '@lucide/svelte/icons/send';
	import type { ComposeDraft } from './mail-types';
	import type { MailComposeFocusField } from './mail-page-controller-types';
	import type { mailText } from './text';

	type Props = {
		open: boolean;
		composeDraft: ComposeDraft;
		composeMessage: string;
		isSending: boolean;
		fromAddress: string;
		focusField: MailComposeFocusField;
		text: (typeof mailText)['ko'];
		sendMessage: () => void | Promise<void>;
	};

	let {
		open = $bindable(false),
		composeDraft = $bindable(),
		composeMessage,
		isSending,
		fromAddress,
		focusField,
		text,
		sendMessage
	}: Props = $props();

	const fieldRowClass = 'flex items-center gap-3 border-b px-4';
	const labelClass = 'w-20 shrink-0 text-xs font-normal text-muted-foreground';
	const fieldClass = 'h-11 rounded-none border-0 bg-transparent px-0 shadow-none focus-visible:ring-0 dark:bg-transparent';

	let isCopyShown = $state(false);
	let recipientInput = $state<HTMLInputElement | null>(null);
	let bodyTextarea = $state<HTMLTextAreaElement | null>(null);

	const canSend = $derived(!isSending && composeDraft.to.trim() !== '' && (composeDraft.subject.trim() !== '' || composeDraft.body.trim() !== ''));

	function focusRequestedField(event: Event) {
		event.preventDefault();
		const field = focusField === 'body' ? bodyTextarea : recipientInput;
		field?.focus();
	}

	function submitCompose(event: SubmitEvent) {
		event.preventDefault();
		sendMessage();
	}
</script>

<Sheet.Root bind:open>
	<Sheet.Content class="flex w-full flex-col gap-0 p-0 sm:max-w-2xl" onOpenAutoFocus={focusRequestedField}>
		<Sheet.Header class="gap-1 border-b p-4">
			<Sheet.Title class="text-base">{text.composeSheet.title}</Sheet.Title>
			<Sheet.Description class="text-xs">{fromAddress}</Sheet.Description>
		</Sheet.Header>

		<form class="flex min-h-0 flex-1 flex-col" onsubmit={submitCompose}>
			<div class={fieldRowClass}>
				<Label for="mail-compose-to" class={labelClass}>{text.to}</Label>
				<Input id="mail-compose-to" class={fieldClass} bind:ref={recipientInput} bind:value={composeDraft.to} placeholder="name@example.com" />
				{#if !isCopyShown}
					<Button type="button" variant="ghost" size="sm" class="shrink-0 text-xs text-muted-foreground" onclick={() => (isCopyShown = true)}>
						{text.fields.cc} · {text.fields.bcc}
					</Button>
				{/if}
			</div>

			{#if isCopyShown}
				<div class={fieldRowClass}>
					<Label for="mail-compose-cc" class={labelClass}>{text.fields.cc}</Label>
					<Input id="mail-compose-cc" class={fieldClass} bind:value={composeDraft.cc} />
				</div>
				<div class={fieldRowClass}>
					<Label for="mail-compose-bcc" class={labelClass}>{text.fields.bcc}</Label>
					<Input id="mail-compose-bcc" class={fieldClass} bind:value={composeDraft.bcc} />
				</div>
			{/if}

			<div class={fieldRowClass}>
				<Label for="mail-compose-subject" class={labelClass}>{text.fields.subject}</Label>
				<Input id="mail-compose-subject" class="{fieldClass} font-medium" bind:value={composeDraft.subject} />
			</div>

			<Label for="mail-compose-body" class="sr-only">{text.fields.body}</Label>
			<Textarea
				id="mail-compose-body"
				class="min-h-0 flex-1 resize-none rounded-none border-0 bg-transparent p-4 text-sm leading-6 shadow-none focus-visible:ring-0 dark:bg-transparent"
				bind:ref={bodyTextarea}
				bind:value={composeDraft.body}
			/>

			<Sheet.Footer class="flex-row items-center justify-between gap-3 border-t p-4">
				<p class="min-w-0 flex-1 text-xs leading-5 text-destructive">{composeMessage}</p>
				<Button type="submit" size="lg" class="shrink-0 gap-2" disabled={!canSend}>
					<SendIcon />
					{isSending ? text.composeSheet.sending : text.composeSheet.send}
				</Button>
			</Sheet.Footer>
		</form>
	</Sheet.Content>
</Sheet.Root>

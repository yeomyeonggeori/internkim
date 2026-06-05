<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Textarea } from '$lib/components/ui/textarea';
	import SendIcon from '@lucide/svelte/icons/send';
	import type { ComposeDraft } from './mail-types';
	import type { mailText } from './text';

	type Props = {
		open: boolean;
		composeDraft: ComposeDraft;
		composeMessage: string;
		isSending: boolean;
		fromAddress: string;
		text: (typeof mailText)['ko'];
		sendMessage: () => void | Promise<void>;
	};

	let {
		open = $bindable(false),
		composeDraft = $bindable(),
		composeMessage,
		isSending,
		fromAddress,
		text,
		sendMessage
	}: Props = $props();
</script>

<Sheet.Root bind:open>
	<Sheet.Content class="w-full overflow-y-auto sm:max-w-2xl">
		<Sheet.Header>
			<Sheet.Title>{text.composeSheet.title}</Sheet.Title>
			<Sheet.Description>{fromAddress}</Sheet.Description>
		</Sheet.Header>
		<form class="grid gap-4 px-4 pb-4" onsubmit={(event) => { event.preventDefault(); sendMessage(); }}>
			<div class="space-y-2">
				<Label for="mail-compose-to">{text.to}</Label>
				<Input id="mail-compose-to" bind:value={composeDraft.to} placeholder="name@example.com" />
			</div>
			<div class="grid gap-3 sm:grid-cols-2">
				<div class="space-y-2">
					<Label for="mail-compose-cc">{text.fields.cc}</Label>
					<Input id="mail-compose-cc" bind:value={composeDraft.cc} />
				</div>
				<div class="space-y-2">
					<Label for="mail-compose-bcc">{text.fields.bcc}</Label>
					<Input id="mail-compose-bcc" bind:value={composeDraft.bcc} />
				</div>
			</div>
			<div class="space-y-2">
				<Label for="mail-compose-subject">{text.fields.subject}</Label>
				<Input id="mail-compose-subject" bind:value={composeDraft.subject} />
			</div>
			<div class="space-y-2">
				<Label for="mail-compose-body">{text.fields.body}</Label>
				<Textarea id="mail-compose-body" class="min-h-72 resize-none" bind:value={composeDraft.body} />
			</div>
			{#if composeMessage}
				<p class="rounded-md border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">{composeMessage}</p>
			{/if}
			<Sheet.Footer>
				<Button type="submit" class="gap-2" disabled={isSending || !composeDraft.to.trim() || (!composeDraft.subject.trim() && !composeDraft.body.trim())}>
					<SendIcon />
					{isSending ? text.composeSheet.sending : text.composeSheet.send}
				</Button>
			</Sheet.Footer>
		</form>
	</Sheet.Content>
</Sheet.Root>

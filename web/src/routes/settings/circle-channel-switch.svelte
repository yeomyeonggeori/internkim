<script lang="ts">
	import { toast } from 'svelte-sonner';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Field from '$lib/components/ui/field';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Switch } from '$lib/components/ui/switch';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { invokeTool } from '$lib/public-api-call';
	import { circleName } from '$lib/data-room/model';
	import type { ListedCircle } from '$lib/data-room/schemas';
	import { dataRoomText } from '$lib/data-room/text';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	let { circle, onChanged }: { circle: ListedCircle; onChanged: () => Promise<void> } = $props();
	const text = createPageText(dataRoomText);
	const fieldID = $props.id();
	let isAdding = $state(false);
	let isSaving = $state(false);
	let channelName = $state('');

	const hasChannel = $derived(circle.channelName !== null);
	const status = $derived.by(() => {
		if (circle.channelName === null) return text.circleChannelDescription;
		if (circle.channelID === null) return text.circleChannelOpening.replace('{name}', circle.channelName);
		return text.circleChannelKept.replace('{name}', circle.channelName);
	});

	function ask(isWanted: boolean) {
		if (isWanted) askToAdd();
		else askToDelete();
	}

	function askToAdd() {
		channelName = circleName(circle, currentLocale.value);
		isAdding = true;
	}

	async function add(event: SubmitEvent) {
		event.preventDefault();
		isSaving = true;
		try {
			await invokeTool('circle_channel_add', { circleID: circle.id, name: channelName });
			await onChanged();
			isAdding = false;
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.circleChannelFailed);
		} finally {
			isSaving = false;
		}
	}

	function askToDelete() {
		const name = circle.channelName ?? '';
		confirmDelete({
			title: text.deleteCircleChannel,
			description: text.deleteCircleChannelDescription.replace('{name}', name),
			input: { confirmationText: name, placeholder: name },
			confirm: { text: text.delete },
			cancel: { text: text.cancel },
			onConfirm: () => remove(name)
		});
	}

	async function remove(confirmedName: string) {
		try {
			await invokeTool('circle_channel_delete', { circleID: circle.id, name: confirmedName });
			await onChanged();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.circleChannelFailed);
		}
	}
</script>

<Field.Field orientation="horizontal"
	><Field.Content
		><Field.Label for="{fieldID}-channel">{text.circleChannel}</Field.Label><Field.Description
			>{status}</Field.Description
		></Field.Content
	><Switch id="{fieldID}-channel" bind:checked={() => hasChannel, ask} /></Field.Field
>

<Dialog.Root bind:open={() => isAdding, (isOpen) => { if (!isSaving) isAdding = isOpen; }}
	><Dialog.Content class="sm:max-w-md"
		><Dialog.Header
			><Dialog.Title>{text.addCircleChannel}</Dialog.Title><Dialog.Description
				>{text.addCircleChannelDescription}</Dialog.Description
			></Dialog.Header
		>
		<form onsubmit={add} class="grid gap-4">
			<Field.Field
				><Field.Label for="{fieldID}-channel-name">{text.circleChannelName}</Field.Label><Input
					id="{fieldID}-channel-name"
					required
					bind:value={channelName}
					disabled={isSaving}
				/></Field.Field
			>
			<Dialog.Footer
				><Button type="button" variant="outline" disabled={isSaving} onclick={() => (isAdding = false)}
					>{text.cancel}</Button
				><Button type="submit" disabled={isSaving || channelName.trim() === ''}
					>{#if isSaving}<Spinner />{/if}{text.create}</Button
				></Dialog.Footer
			>
		</form>
	</Dialog.Content></Dialog.Root
>

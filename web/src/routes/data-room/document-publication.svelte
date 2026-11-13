<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { Switch } from '$lib/components/ui/switch';
	import { invokeTool } from '$lib/public-api-call';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { dataRoomText } from '$lib/data-room/text';
	import type { DataRoomDocument } from '$lib/data-room/browser';

	type Props = { document: DataRoomDocument; isPublic: boolean; onChange: (documentID: string, isPublic: boolean) => void };

	let { document, isPublic, onChange }: Props = $props();
	const text = createPageText(dataRoomText);
	const fieldID = $props.id();
	const isInbox = $derived(document.categoryCode === 'X');
	let isSaving = $state(false);

	async function publish(shouldBePublic: boolean) {
		isSaving = true;
		try {
			await invokeTool('dataroom_publication_update', { documentHint: document.documentID, isPublic: shouldBePublic });
			onChange(document.documentID, shouldBePublic);
			toast.success(shouldBePublic ? text.published : text.withdrawn);
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.loadFailed);
		} finally {
			isSaving = false;
		}
	}
</script>

<section class="grid gap-2">
	<h3 class="text-xs font-medium tracking-wide text-muted-foreground uppercase">{text.publication}</h3>
	<div class="flex items-center justify-between gap-3">
		<label for="{fieldID}-publish" class="text-sm">{text.publish}</label>
		<Switch id="{fieldID}-publish" checked={isPublic} disabled={isSaving || (isInbox && !isPublic)}
			onCheckedChange={(checked) => publish(checked)} />
	</div>
	<p class="text-xs text-muted-foreground">{isInbox && !isPublic ? text.publishNeedsCategory : text.publishDescription}</p>
</section>

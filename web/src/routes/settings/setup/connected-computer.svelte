<script lang="ts">
	import * as Item from '$lib/components/ui/item';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { hostSetupText } from './text';

	const text = createPageText(hostSetupText);
</script>

<Item.Root variant="outline">
	<Item.Content>
		<Item.Title>{text.connectedComputer}</Item.Title>
		<Item.Description>{text.connectedComputerStatus}</Item.Description>
	</Item.Content>
</Item.Root>

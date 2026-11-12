<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { stepHeading } from './step-heading';
	import { hostSetupText } from './text';

	let { step }: { step?: number } = $props();

	const text = createPageText(hostSetupText);
</script>

<Card.Root aria-hidden="true">
	<Card.Header>
		<Card.Title>{stepHeading(text.computer, step)}</Card.Title>
		<Card.Description>{text.computerDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid min-w-0 gap-6">
		<Skeleton class="h-16 w-full rounded-lg" />
		<span class="inline-flex min-h-11 items-center justify-self-start text-sm underline">{text.otherComputer}</span>
	</Card.Content>
	<Card.Footer class="flex-col items-start gap-3">
		<Button variant="outline" disabled>{text.check}</Button>
	</Card.Footer>
</Card.Root>

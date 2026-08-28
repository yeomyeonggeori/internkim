<script lang="ts">
	import * as Card from '$lib/components/ui/card';

	type Props = {
		label: string;
		value: number;
		subvalue: string;
	};

	let { label, value, subvalue }: Props = $props();
</script>

<Card.Root size="sm">
	<Card.Header>
		<Card.Description class="text-xs uppercase tracking-wide">{label}</Card.Description>
	</Card.Header>
	<Card.Content>
		<div class="text-3xl font-semibold leading-none">{value}</div>
		<div class="mt-2 text-xs text-muted-foreground">{subvalue}</div>
	</Card.Content>
</Card.Root>

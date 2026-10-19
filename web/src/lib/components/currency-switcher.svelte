<script lang="ts">
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { buttonVariants } from '$lib/components/ui/button';
	import { interimCurrencyCatalogue } from '$lib/currency/currency-catalogue';
	import { displayCurrency, setDisplayCurrency } from '$lib/currency/display-currency.svelte';

	let { label, asRecorded }: { label: string; asRecorded: string } = $props();
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger class={buttonVariants({ variant: 'ghost', size: 'sm' })} aria-label={label}>
		<span class="tabular-nums">{displayCurrency.value || asRecorded}</span><ChevronDownIcon class="text-muted-foreground size-3.5" />
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="end">
		<DropdownMenu.RadioGroup value={displayCurrency.value} onValueChange={setDisplayCurrency}>
			<DropdownMenu.RadioItem value="">{asRecorded}</DropdownMenu.RadioItem>
			<DropdownMenu.Separator />
			{#each interimCurrencyCatalogue as currency (currency.code)}
				<DropdownMenu.RadioItem value={currency.code}>
					<span class="w-9 font-medium tabular-nums">{currency.code}</span><span class="text-muted-foreground">{currency.name}</span>
				</DropdownMenu.RadioItem>
			{/each}
		</DropdownMenu.RadioGroup>
	</DropdownMenu.Content>
</DropdownMenu.Root>

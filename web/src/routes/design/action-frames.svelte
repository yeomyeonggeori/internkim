<script lang="ts">
	import { Button, buttonVariants } from '$lib/components/ui/button/index.js';
	import * as ButtonGroup from '$lib/components/ui/button-group/index.js';
	import { buttonGroupVariants } from '$lib/components/ui/button-group/index.js';
	import { CopyButton } from '$lib/components/ui/copy-button/index.js';
	import * as Kbd from '$lib/components/ui/kbd/index.js';
	import LanguageSwitcher from '$lib/components/ui/language-switcher/language-switcher.svelte';
	import LightSwitch from '$lib/components/ui/light-switch/light-switch.svelte';
	import { Spinner } from '$lib/components/ui/spinner/index.js';
	import { Toggle, toggleVariants } from '$lib/components/ui/toggle/index.js';
	import * as ToggleGroup from '$lib/components/ui/toggle-group/index.js';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { localeOptions } from '$lib/i18n/locale.svelte';
	import BoldIcon from '@lucide/svelte/icons/bold';
	import ItalicIcon from '@lucide/svelte/icons/italic';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import SendIcon from '@lucide/svelte/icons/send';
	import UnderlineIcon from '@lucide/svelte/icons/underline';
	import DesignFrame from './design-frame.svelte';
	import DesignSection from './design-section.svelte';
	import { designText } from './text';
	import { variantAxes, variantValues } from './workspace.svelte';

	let { pageID }: { pageID: string } = $props();

	const text = createPageText(designText);
	const buttonKinds = variantValues(buttonVariants.variants.variant);
	const buttonSizes = variantValues(buttonVariants.variants.size).filter((size) => !size.startsWith('icon'));
	const iconSizes = variantValues(buttonVariants.variants.size).filter((size) => size.startsWith('icon'));
	const toggleKinds = variantValues(toggleVariants.variants.variant);
	const toggleSizes = variantValues(toggleVariants.variants.size);
	const languages = localeOptions.map((option) => ({ code: option.value, label: option.label }));
</script>

<DesignSection {pageID} title={text.actionSection}>
	<DesignFrame id="button" name="Button" source="ui/button" variants={variantAxes(buttonVariants.variants)} class="w-[40rem]">
		{#snippet children()}
			<div class="grid grid-cols-[5.5rem_1fr] items-center gap-x-4 gap-y-3">
				{#each buttonKinds as variant (variant)}
					<span class="text-muted-foreground font-mono text-[10px]">{variant}</span>
					<div class="flex flex-wrap items-center gap-2">
						<Button {variant}>{text.sample.save}</Button>
						<Button {variant}><PlusIcon data-icon="inline-start" />{text.sample.add}</Button>
						<Button {variant} size="icon" aria-label={text.sample.add}><PlusIcon /></Button>
						<Button {variant} disabled>{text.sample.disabled}</Button>
					</div>
				{/each}
				<span class="text-muted-foreground font-mono text-[10px]">size</span>
				<div class="flex flex-wrap items-center gap-2">
					{#each buttonSizes as size (size)}
						<Button variant="outline" {size}>{size}</Button>
					{/each}
				</div>
				<span class="text-muted-foreground font-mono text-[10px]">icon size</span>
				<div class="flex flex-wrap items-center gap-2">
					{#each iconSizes as size (size)}
						<Button variant="outline" {size} aria-label={size}><SendIcon /></Button>
					{/each}
				</div>
				<span class="text-muted-foreground font-mono text-[10px]">state</span>
				<div class="flex flex-wrap items-center gap-2">
					<Button disabled><Spinner data-icon="inline-start" />{text.sample.saving}</Button>
					<Button variant="outline" aria-invalid="true">{text.sample.invalid}</Button>
					<Button variant="outline" aria-expanded="true">{text.sample.expanded}</Button>
				</div>
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="button-group" name="ButtonGroup" source="ui/button-group" variants={variantAxes(buttonGroupVariants.variants)}>
		{#snippet children()}
			<ButtonGroup.Root>
				<Button variant="outline">{text.sample.previous}</Button>
				<Button variant="outline">{text.sample.today}</Button>
				<Button variant="outline">{text.sample.next}</Button>
			</ButtonGroup.Root>
			<ButtonGroup.Root>
				<ButtonGroup.Text>https://</ButtonGroup.Text>
				<Button variant="outline">example.com</Button>
				<ButtonGroup.Separator />
				<Button variant="outline" size="icon" aria-label={text.sample.add}><PlusIcon /></Button>
			</ButtonGroup.Root>
			<ButtonGroup.Root orientation="vertical" class="w-fit">
				<Button variant="outline" size="icon" aria-label="+"><PlusIcon /></Button>
				<Button variant="outline" size="icon" aria-label="send"><SendIcon /></Button>
			</ButtonGroup.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="toggle" name="Toggle" source="ui/toggle" variants={variantAxes(toggleVariants.variants)}>
		{#snippet children()}
			{#each toggleKinds as variant (variant)}
				<div class="flex flex-wrap items-center gap-2">
					{#each toggleSizes as size (size)}
						<Toggle {variant} {size} aria-label="bold"><BoldIcon /></Toggle>
					{/each}
					<Toggle {variant} pressed aria-label="italic"><ItalicIcon /></Toggle>
					<Toggle {variant} disabled aria-label="underline"><UnderlineIcon /></Toggle>
					<span class="text-muted-foreground font-mono text-[10px]">{variant}</span>
				</div>
			{/each}
		{/snippet}
	</DesignFrame>

	<DesignFrame id="toggle-group" name="ToggleGroup" source="ui/toggle-group">
		{#snippet children()}
			<ToggleGroup.Root type="single" value="week" variant="outline">
				<ToggleGroup.Item value="day">{text.sample.day}</ToggleGroup.Item>
				<ToggleGroup.Item value="week">{text.sample.week}</ToggleGroup.Item>
				<ToggleGroup.Item value="month">{text.sample.month}</ToggleGroup.Item>
			</ToggleGroup.Root>
			<ToggleGroup.Root type="multiple" value={['bold']}>
				<ToggleGroup.Item value="bold" aria-label="bold"><BoldIcon /></ToggleGroup.Item>
				<ToggleGroup.Item value="italic" aria-label="italic"><ItalicIcon /></ToggleGroup.Item>
				<ToggleGroup.Item value="underline" aria-label="underline"><UnderlineIcon /></ToggleGroup.Item>
			</ToggleGroup.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="copy-button" name="CopyButton" source="ui/copy-button">
		{#snippet children()}
			<div class="flex items-center gap-2">
				<CopyButton text="https://example.com" />
				<CopyButton text="https://example.com" variant="outline">{text.sample.copyLink}</CopyButton>
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="light-switch" name="LightSwitch" source="ui/light-switch" note={text.lightSwitchNote}>
		{#snippet children()}
			<div class="flex items-center gap-2">
				<LightSwitch />
				<LightSwitch variant="ghost" />
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="language-switcher" name="LanguageSwitcher" source="ui/language-switcher">
		{#snippet children()}
			<div class="flex items-center gap-2">
				<LanguageSwitcher {languages} value="ko" />
				<LanguageSwitcher {languages} value="en" variant="ghost" />
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="kbd" name="Kbd" source="ui/kbd">
		{#snippet children()}
			<div class="flex flex-wrap items-center gap-3 text-sm">
				<Kbd.Group><Kbd.Root>⌘</Kbd.Root><Kbd.Root>K</Kbd.Root></Kbd.Group>
				<Kbd.Root>Esc</Kbd.Root>
				<Kbd.Group><Kbd.Root>Shift</Kbd.Root><Kbd.Root>Enter</Kbd.Root></Kbd.Group>
			</div>
		{/snippet}
	</DesignFrame>
</DesignSection>

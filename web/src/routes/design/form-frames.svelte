<script lang="ts">
	import { getLocalTimeZone, today } from '@internationalized/date';
	import { buttonVariants } from '$lib/components/ui/button/index.js';
	import { Calendar } from '$lib/components/ui/calendar/index.js';
	import { Checkbox } from '$lib/components/ui/checkbox/index.js';
	import * as Field from '$lib/components/ui/field/index.js';
	import * as FileDropZone from '$lib/components/ui/file-drop-zone/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import * as InputGroup from '$lib/components/ui/input-group/index.js';
	import { inputGroupAddonVariants } from '$lib/components/ui/input-group/input-group-addon.svelte';
	import * as InputOTP from '$lib/components/ui/input-otp/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { RangeCalendar } from '$lib/components/ui/range-calendar/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import { Slider } from '$lib/components/ui/slider/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { TagsInput } from '$lib/components/ui/tags-input/index.js';
	import { Textarea } from '$lib/components/ui/textarea/index.js';
	import { fieldVariants } from '$lib/components/ui/field/field.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import AtSignIcon from '@lucide/svelte/icons/at-sign';
	import SearchIcon from '@lucide/svelte/icons/search';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import DesignFrame from './design-frame.svelte';
	import DesignSection from './design-section.svelte';
	import { designText } from './text';
	import { variantAxes } from './workspace.svelte';

	let { pageID }: { pageID: string } = $props();

	const text = createPageText(designText);
	const calendarLocale = $derived(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US');
	const day = today(getLocalTimeZone());
	const doNothing = () => {};
</script>

<DesignSection {pageID} title={text.formSection}>
	<DesignFrame id="input" name="Input" source="ui/input">
		{#snippet children()}
			<Input placeholder={text.sample.placeholder} />
			<Input value={text.sample.filled} />
			<Input aria-invalid="true" value={text.sample.invalid} />
			<Input disabled placeholder={text.sample.disabled} />
		{/snippet}
	</DesignFrame>

	<DesignFrame id="textarea" name="Textarea" source="ui/textarea">
		{#snippet children()}
			<Textarea placeholder={text.sample.placeholder} />
			<Textarea aria-invalid="true" value={text.sample.invalid} />
		{/snippet}
	</DesignFrame>

	<DesignFrame id="input-group" name="InputGroup" source="ui/input-group" variants={variantAxes(inputGroupAddonVariants.variants)}>
		{#snippet children()}
			<InputGroup.Root>
				<InputGroup.Addon><SearchIcon /></InputGroup.Addon>
				<InputGroup.Input placeholder={text.sample.search} />
				<InputGroup.Addon align="inline-end"><InputGroup.Text>12</InputGroup.Text></InputGroup.Addon>
			</InputGroup.Root>
			<InputGroup.Root>
				<InputGroup.Addon><AtSignIcon /></InputGroup.Addon>
				<InputGroup.Input value="sample" />
				<InputGroup.Addon align="inline-end"><InputGroup.Text>@example.com</InputGroup.Text></InputGroup.Addon>
			</InputGroup.Root>
			<InputGroup.Root>
				<InputGroup.Textarea placeholder={text.sample.message} />
				<InputGroup.Addon align="block-end" class="justify-end">
					<InputGroup.Button variant="default" size="icon-sm" class="rounded-full" aria-label={text.sample.send}><ArrowUpIcon /></InputGroup.Button>
				</InputGroup.Addon>
			</InputGroup.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="input-otp" name="InputOTP" source="ui/input-otp">
		{#snippet children()}
			<InputOTP.Root maxlength={6} value="4821">
				{#snippet children({ cells })}
					<InputOTP.Group>
						{#each cells.slice(0, 3) as cell (cell)}<InputOTP.Slot {cell} />{/each}
					</InputOTP.Group>
					<InputOTP.Separator />
					<InputOTP.Group>
						{#each cells.slice(3) as cell (cell)}<InputOTP.Slot {cell} />{/each}
					</InputOTP.Group>
				{/snippet}
			</InputOTP.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="field" name="Field · Label" source="ui/field" variants={variantAxes(fieldVariants.variants)} class="w-96">
		{#snippet children(theme)}
			<Field.Set>
				<Field.Legend>{text.sample.profile}</Field.Legend>
				<Field.Description>{text.sample.profileDescription}</Field.Description>
				<Field.Group>
					<Field.Field>
						<Field.Label for={`design-name-${theme}`}>{text.sample.name}</Field.Label>
						<Input id={`design-name-${theme}`} value="이샘플" />
						<Field.Description>{text.sample.nameDescription}</Field.Description>
					</Field.Field>
					<Field.Field data-invalid="true">
						<Field.Label for={`design-email-${theme}`}>{text.sample.email}</Field.Label>
						<Input id={`design-email-${theme}`} aria-invalid="true" value="sample@" />
						<Field.Error errors={[{ message: text.sample.emailError }]} />
					</Field.Field>
					<Field.Separator />
					<Field.Field orientation="horizontal">
						<Checkbox id={`design-notify-${theme}`} checked />
						<Field.Label for={`design-notify-${theme}`}>{text.sample.notify}</Field.Label>
					</Field.Field>
				</Field.Group>
			</Field.Set>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="label" name="Label" source="ui/label">
		{#snippet children(theme)}
			<div class="grid gap-2">
				<Label for={`design-label-${theme}`}>{text.sample.name}</Label>
				<Input id={`design-label-${theme}`} placeholder={text.sample.placeholder} />
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="checkbox" name="Checkbox" source="ui/checkbox">
		{#snippet children()}
			<div class="flex items-center gap-4">
				<Checkbox aria-label="unchecked" />
				<Checkbox checked aria-label="checked" />
				<Checkbox indeterminate aria-label="indeterminate" />
				<Checkbox disabled aria-label="disabled" />
				<Checkbox aria-invalid="true" aria-label="invalid" />
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="switch" name="Switch" source="ui/switch">
		{#snippet children()}
			<div class="flex items-center gap-4">
				<Switch aria-label="off" />
				<Switch checked aria-label="on" />
				<Switch disabled aria-label="disabled" />
				<Switch size="sm" checked aria-label="small" />
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="slider" name="Slider" source="ui/slider">
		{#snippet children()}
			<Slider type="single" value={40} />
			<Slider type="multiple" value={[20, 70]} />
			<Slider type="single" value={60} size="lg" />
		{/snippet}
	</DesignFrame>

	<DesignFrame id="select" name="Select" source="ui/select" note={text.selectNote}>
		{#snippet children()}
			{#each [{ value: '' }, { value: 'week' }] as state (state.value)}
				<Select.Root type="single" value={state.value}>
					<Select.Trigger class="w-48">{state.value ? text.sample.week : text.sample.period}</Select.Trigger>
					<Select.Content>
						<Select.Group>
							<Select.Label>{text.sample.period}</Select.Label>
							<Select.Item value="day">{text.sample.day}</Select.Item>
							<Select.Item value="week">{text.sample.week}</Select.Item>
							<Select.Item value="month">{text.sample.month}</Select.Item>
							<Select.Separator />
							<Select.Item value="year" disabled>{text.sample.year}</Select.Item>
						</Select.Group>
					</Select.Content>
				</Select.Root>
			{/each}
			<Select.Root type="single" disabled>
				<Select.Trigger class="w-48">{text.sample.disabled}</Select.Trigger>
			</Select.Root>
			<Select.Root type="single">
				<Select.Trigger class="w-48" aria-invalid="true">{text.sample.invalid}</Select.Trigger>
			</Select.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="tags-input" name="TagsInput" source="ui/tags-input">
		{#snippet children()}
			<TagsInput value={[text.sample.tagOne, text.sample.tagTwo]} placeholder={text.sample.addTag} />
		{/snippet}
	</DesignFrame>

	<DesignFrame id="calendar" name="Calendar" source="ui/calendar">
		{#snippet children()}
			<Calendar type="single" value={day} locale={calendarLocale} class="rounded-lg border" />
		{/snippet}
	</DesignFrame>

	<DesignFrame id="range-calendar" name="RangeCalendar" source="ui/range-calendar">
		{#snippet children()}
			<RangeCalendar value={{ start: day.subtract({ days: 2 }), end: day.add({ days: 3 }) }} locale={calendarLocale} class="rounded-lg border" />
		{/snippet}
	</DesignFrame>

	<DesignFrame id="file-drop-zone" name="FileDropZone" source="ui/file-drop-zone">
		{#snippet children()}
			<FileDropZone.Root onUpload={async () => doNothing()}>
				<FileDropZone.Trigger>
					<div class="hover:bg-accent/25 flex h-40 flex-col items-center justify-center gap-2 rounded-lg border border-dashed p-6">
						<UploadIcon class="text-muted-foreground size-6" />
						<span class="text-sm font-medium">{text.sample.dropHint}</span>
						<span class={buttonVariants({ variant: 'outline', size: 'sm' })}>{text.sample.chooseFile}</span>
					</div>
				</FileDropZone.Trigger>
			</FileDropZone.Root>
		{/snippet}
	</DesignFrame>
</DesignSection>

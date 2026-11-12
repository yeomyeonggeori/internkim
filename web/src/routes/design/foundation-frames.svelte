<script lang="ts">
	import { browser } from '$app/environment';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import { Spinner } from '$lib/components/ui/spinner/index.js';
	import ActivityMarker from '$lib/components/channel/activity-marker.svelte';
	import TypingDots from '$lib/components/channel/typing-dots.svelte';
	import GradientAvatar from '$lib/components/gradient-avatar.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import BellIcon from '@lucide/svelte/icons/bell';
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import CheckIcon from '@lucide/svelte/icons/check';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import EllipsisVerticalIcon from '@lucide/svelte/icons/ellipsis-vertical';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import FolderIcon from '@lucide/svelte/icons/folder';
	import InfoIcon from '@lucide/svelte/icons/info';
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
	import MailIcon from '@lucide/svelte/icons/mail';
	import MessageCircleIcon from '@lucide/svelte/icons/message-circle';
	import PaperclipIcon from '@lucide/svelte/icons/paperclip';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import ReplyIcon from '@lucide/svelte/icons/reply';
	import SearchIcon from '@lucide/svelte/icons/search';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import SmilePlusIcon from '@lucide/svelte/icons/smile-plus';
	import SquareCheckIcon from '@lucide/svelte/icons/square-check';
	import TrashIcon from '@lucide/svelte/icons/trash-2';
	import UsersIcon from '@lucide/svelte/icons/users';
	import XIcon from '@lucide/svelte/icons/x';
	import ColorSwatch from './color-swatch.svelte';
	import DesignFrame from './design-frame.svelte';
	import DesignSection from './design-section.svelte';
	import Measured from './measured.svelte';
	import { designText } from './text';
	import { baseRadius, colorTokens, fontFamily, radiusTokens } from './theme-tokens';

	let { pageID }: { pageID: string } = $props();

	const text = createPageText(designText);

	const typeScale = ['text-[11px]', 'text-xs', 'text-sm', 'text-base', 'text-lg', 'text-xl', 'text-2xl', 'text-3xl'];
	const spacingScale = ['w-0.5', 'w-1', 'w-1.5', 'w-2', 'w-2.5', 'w-3', 'w-4', 'w-5', 'w-6', 'w-8', 'w-10', 'w-12', 'w-16', 'w-24'];
	const shadowScale = ['shadow-xs', 'shadow-sm', 'shadow-md', 'shadow-lg', 'shadow-xl'];
	const iconSizes = ['size-3', 'size-3.5', 'size-4', 'size-5', 'size-6'];
	const iconTones = ['text-foreground', 'text-muted-foreground', 'text-brand', 'text-success', 'text-warning', 'text-destructive'];
	const icons = [
		ArrowUpIcon, BellIcon, CalendarIcon, CheckIcon, CircleAlertIcon, EllipsisVerticalIcon, FileTextIcon, FolderIcon,
		InfoIcon, MailIcon, MessageCircleIcon, PaperclipIcon, PlusIcon, ReplyIcon, SearchIcon, SettingsIcon,
		SmilePlusIcon, SquareCheckIcon, TrashIcon, UsersIcon, XIcon
	];

	const fontWeights = $derived(
		browser
			? [...new Set([...document.fonts].filter((face) => fontFamily.includes(face.family.replaceAll(/['"]/g, ''))).map((face) => face.weight))].sort()
			: []
	);

	let enterReplay = $state(0);
</script>

<DesignSection {pageID} title={text.colorSection} description={text.colorSectionDescription}>
	<DesignFrame id="color" name={text.colorFrame} note={text.colorNote} class="w-[46rem]">
		{#snippet children(theme)}
			<div class="grid grid-cols-[repeat(auto-fill,minmax(7.5rem,1fr))] gap-x-3 gap-y-5">
				{#each colorTokens as token (token.name)}
					<ColorSwatch {token} {theme} />
				{/each}
			</div>
		{/snippet}
	</DesignFrame>
</DesignSection>

<DesignSection {pageID} title={text.typeSection} description={text.typeSectionDescription}>
	<DesignFrame id="type-family" name={text.typeFamilyFrame} note={text.typeFamilyNote} class="w-80">
		{#snippet children()}
			<p class="text-5xl leading-none font-semibold">가 Aa</p>
			<p class="font-mono text-xs">{fontFamily}</p>
			<div class="grid gap-1.5">
				{#each fontWeights as weight (weight)}
					<div class="flex items-baseline justify-between gap-3">
						<span class="text-lg" style:font-weight={weight}>{text.typeSample}</span>
						<span class="text-muted-foreground font-mono text-[10px]">{weight}</span>
					</div>
				{/each}
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="type-scale" name={text.typeScaleFrame} note={text.typeScaleNote} class="w-[34rem]">
		{#snippet children()}
			{#each typeScale as size (size)}
				<div class="grid grid-cols-[6.5rem_1fr] items-center gap-3">
					<span class="text-muted-foreground font-mono text-[10px]">{size}</span>
					<Measured measure="font" class={`${size} truncate`}>{text.typeSample}</Measured>
				</div>
			{/each}
		{/snippet}
	</DesignFrame>

	<DesignFrame id="type-prose" name={text.typeProseFrame} note={text.typeProseNote} class="w-80">
		{#snippet children()}
			<div class="chat-markdown prose prose-sm dark:prose-invert">
				<h3>{text.proseHeading}</h3>
				<p>{text.proseParagraph} <a href="#type-prose">{text.proseLink}</a> <code>tools/verify</code></p>
				<ul>
					<li>{text.proseItemOne}</li>
					<li>{text.proseItemTwo}</li>
				</ul>
				<blockquote>{text.proseQuote}</blockquote>
			</div>
		{/snippet}
	</DesignFrame>
</DesignSection>

<DesignSection {pageID} title={text.layoutSection} description={text.layoutSectionDescription}>
	<DesignFrame id="spacing" name={text.spacingFrame} note={text.spacingNote} class="w-72">
		{#snippet children()}
			{#each spacingScale as width (width)}
				<div class="grid grid-cols-[3.5rem_1fr] items-center gap-3">
					<span class="text-muted-foreground font-mono text-[10px]">{width}</span>
					<Measured measure="width" class={`${width} bg-brand/70 h-3 rounded-[1px]`} />
				</div>
			{/each}
		{/snippet}
	</DesignFrame>

	<DesignFrame id="radius" name={text.radiusFrame} note={text.radiusNote} class="w-80">
		{#snippet children()}
			<p class="text-muted-foreground font-mono text-[10px]">--radius: {baseRadius}</p>
			<div class="grid grid-cols-2 gap-4">
				{#each radiusTokens as token (token.name)}
					<div class="grid gap-1.5">
						<span class="text-xs font-medium">rounded-{token.name}</span>
						<Measured measure="radius" class="bg-muted border-foreground/20 size-14 border" style={`border-radius: var(--radius-${token.name})`} />
						<span class="text-muted-foreground font-mono text-[10px]">{token.definition}</span>
					</div>
				{/each}
				<div class="grid gap-1.5">
					<span class="text-xs font-medium">rounded-full</span>
					<Measured measure="radius" class="bg-muted border-foreground/20 size-14 rounded-full border" />
				</div>
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="elevation" name={text.elevationFrame} note={text.elevationNote} class="w-[30rem]">
		{#snippet children()}
			<div class="grid grid-cols-3 gap-5 sm:grid-cols-5">
				{#each shadowScale as shadow (shadow)}
					<div class="grid justify-items-center gap-2">
						<div class={`${shadow} bg-card size-14 rounded-lg border`}></div>
						<span class="text-muted-foreground font-mono text-[10px]">{shadow}</span>
					</div>
				{/each}
			</div>
			<div class="relative h-28 overflow-hidden rounded-lg">
				<GradientAvatar seed="design-surface" />
				<div class="frosted-surface absolute inset-x-4 bottom-4 rounded-full border px-4 py-2 text-sm">.frosted-surface</div>
			</div>
			<div class="flex flex-wrap items-center gap-4">
				<div class="border-ring ring-ring/50 rounded-lg border px-3 py-1.5 text-sm ring-3">{text.focusRing}</div>
				<div class="border-destructive ring-destructive/20 rounded-lg border px-3 py-1.5 text-sm ring-3">{text.invalidRing}</div>
			</div>
		{/snippet}
	</DesignFrame>
</DesignSection>

<DesignSection {pageID} title={text.motionSection} description={text.motionSectionDescription}>
	<DesignFrame id="motion" name={text.motionFrame} note={text.motionNote} class="w-[34rem]">
		{#snippet children()}
			<div class="grid grid-cols-[8rem_1fr] items-center gap-x-4 gap-y-5 text-sm">
				<span class="text-muted-foreground text-xs">{text.motionDots}</span>
				<div class="bg-muted w-fit rounded-xl px-3 py-2.5"><TypingDots /></div>
				<span class="text-muted-foreground text-xs">{text.motionShimmer}</span>
				<div class="flex h-6 items-center"><ActivityMarker label={text.motionShimmerSample} /></div>
				<span class="text-muted-foreground text-xs">{text.motionSpin}</span>
				<div class="flex items-center gap-4">
					<Spinner />
					<LoaderCircleIcon class="size-3.5 animate-spin motion-reduce:animate-none" />
				</div>
				<span class="text-muted-foreground text-xs">{text.motionPulse}</span>
				<div class="flex items-center gap-2">
					<Skeleton class="size-9 rounded-full" />
					<Skeleton class="h-4 w-32" />
				</div>
				<span class="text-muted-foreground text-xs">{text.motionEnter}</span>
				<div class="flex items-center gap-3">
					{#key enterReplay}
						<div class="bg-popover animate-in fade-in-0 zoom-in-95 rounded-lg border px-3 py-1.5 text-xs shadow-md duration-200">
							{text.motionEnterSample}
						</div>
					{/key}
					<Button variant="outline" size="xs" onclick={() => (enterReplay += 1)}>{text.replay}</Button>
				</div>
			</div>
		{/snippet}
	</DesignFrame>
</DesignSection>

<DesignSection {pageID} title={text.iconSection} description={text.iconSectionDescription}>
	<DesignFrame id="icon-library" name={text.iconLibraryFrame} note={text.iconLibraryNote} class="w-[30rem]">
		{#snippet children()}
			<div class="grid grid-cols-7 gap-4">
				{#each icons as Icon, index (index)}
					<Icon class="size-5" />
				{/each}
			</div>
			<a class="text-muted-foreground hover:text-foreground w-fit text-xs underline underline-offset-3" href="https://lucide.dev/icons" target="_blank" rel="noreferrer">lucide.dev/icons</a>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="icon-sizes" name={text.iconSizesFrame} note={text.iconSizesNote} class="w-[30rem]">
		{#snippet children()}
			<div class="flex flex-wrap items-end gap-6">
				{#each iconSizes as size (size)}
					<div class="grid justify-items-center gap-2">
						<MessageCircleIcon class={size} />
						<span class="text-muted-foreground font-mono text-[10px]">{size}</span>
					</div>
				{/each}
			</div>
			<div class="flex flex-wrap items-center gap-5">
				{#each iconTones as tone (tone)}
					<div class="grid justify-items-center gap-2">
						<CircleAlertIcon class={`${tone} size-5`} />
						<span class="text-muted-foreground font-mono text-[10px]">{tone.replace('text-', '')}</span>
					</div>
				{/each}
			</div>
		{/snippet}
	</DesignFrame>
</DesignSection>

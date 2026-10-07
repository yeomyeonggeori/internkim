<script lang="ts" module>
	export type VideoPlayerVariant = 'message' | 'viewer';
</script>

<script lang="ts">
	import PauseIcon from '@lucide/svelte/icons/pause';
	import PlayIcon from '@lucide/svelte/icons/play';
	import Volume2Icon from '@lucide/svelte/icons/volume-2';
	import VolumeXIcon from '@lucide/svelte/icons/volume-x';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Slider } from '$lib/components/ui/slider/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { formatPlaybackTime, playbackRates } from './video-playback';

	let {
		source,
		width,
		height,
		variant,
		onOpen
	}: {
		source: string;
		width?: number;
		height?: number;
		variant: VideoPlayerVariant;
		onOpen?: () => void;
	} = $props();

	const text = createPageText(channelText);

	let paused = $state(true);
	let currentSeconds = $state(0);
	let durationSeconds = $state(0);
	let playbackRate = $state(1);
	let muted = $state(false);
	let volume = $state(1);

	const knownDuration = $derived(Number.isFinite(durationSeconds) ? durationSeconds : 0);
	const aspectRatio = $derived(width && height ? `${width} / ${height}` : '16 / 9');

	function togglePlaying() {
		paused = !paused;
	}

	function selectPlaybackRate(value: string) {
		playbackRate = Number(value);
	}
</script>

<div
	data-video-player={variant}
	class={variant === 'message'
		? 'relative w-[26rem] max-w-full overflow-hidden rounded-lg bg-black'
		: 'relative z-10 flex w-[min(calc(100vw-3rem),64rem)] flex-col gap-2'}
>
	<div class="relative" style:aspect-ratio={aspectRatio}>
		<video
			src={source}
			preload="metadata"
			playsinline
			class={variant === 'message' ? 'size-full object-contain' : 'max-h-[calc(100dvh-10rem)] size-full rounded-md bg-black object-contain'}
			bind:paused
			bind:currentTime={currentSeconds}
			bind:duration={durationSeconds}
			bind:playbackRate
			bind:muted
			bind:volume
		></video>
		<button
			type="button"
			class="absolute inset-0 flex items-center justify-center"
			aria-label={variant === 'message' && paused ? text.openVideo : paused ? text.playVideo : text.pauseVideo}
			onclick={variant === 'message' && paused && onOpen ? onOpen : togglePlaying}
		>
			{#if paused}
				<span
					class="flex size-14 items-center justify-center rounded-full bg-black/60 text-white backdrop-blur"
					aria-hidden="true"
				>
					<PlayIcon class="size-6 translate-x-0.5" />
				</span>
			{/if}
		</button>
	</div>
	<div
		class={variant === 'message'
			? 'absolute inset-x-0 bottom-0 flex items-center gap-2 bg-black/50 px-3 py-2 text-xs text-white'
			: 'flex items-center gap-3 px-1 text-sm text-white'}
	>
		<button
			type="button"
			class="flex size-7 shrink-0 items-center justify-center rounded-full transition hover:bg-white/20"
			aria-label={paused ? text.playVideo : text.pauseVideo}
			onclick={togglePlaying}
		>
			{#if paused}<PlayIcon class="size-4" />{:else}<PauseIcon class="size-4" />{/if}
		</button>
		<span class="shrink-0 tabular-nums">{formatPlaybackTime(currentSeconds)}</span>
		<Slider
			type="single"
			value={currentSeconds}
			min={0}
			max={knownDuration || 1}
			step={0.1}
			disabled={knownDuration === 0}
			aria-label={text.videoPosition}
			onValueChange={(seconds: number) => (currentSeconds = seconds)}
			class="min-w-0 flex-1"
		/>
		<span class="shrink-0 tabular-nums">{formatPlaybackTime(knownDuration)}</span>
		<DropdownMenu.Root>
			<DropdownMenu.Trigger
				class="shrink-0 rounded px-1.5 py-0.5 tabular-nums transition hover:bg-white/20"
				aria-label={text.playbackSpeed}
			>
				{playbackRate}x
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="end" class="z-(--layer-alert)">
				<DropdownMenu.RadioGroup value={String(playbackRate)} onValueChange={selectPlaybackRate}>
					{#each playbackRates as rate (rate)}
						<DropdownMenu.RadioItem value={String(rate)}>{rate}x</DropdownMenu.RadioItem>
					{/each}
				</DropdownMenu.RadioGroup>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
		<button
			type="button"
			class="flex size-7 shrink-0 items-center justify-center rounded-full transition hover:bg-white/20"
			aria-label={muted ? text.unmuteVideo : text.muteVideo}
			onclick={() => (muted = !muted)}
		>
			{#if muted}<VolumeXIcon class="size-4" />{:else}<Volume2Icon class="size-4" />{/if}
		</button>
		{#if variant === 'viewer'}
			<Slider
				type="single"
				value={muted ? 0 : volume}
				min={0}
				max={1}
				step={0.05}
				aria-label={text.videoVolume}
				onValueChange={(level: number) => {
					volume = level;
					muted = level === 0;
				}}
				class="w-24 shrink-0"
			/>
		{/if}
	</div>
</div>

<div class="flex h-4 items-center gap-1" aria-hidden="true">
	{#each [0, 160, 320] as delayMilliseconds (delayMilliseconds)}
		<span class="typing-dot bg-muted-foreground size-1.5 rounded-full" style:animation-delay="{delayMilliseconds}ms"></span>
	{/each}
</div>

<style>
	@keyframes typing-dot {
		0%,
		60%,
		100% {
			opacity: 0.35;
			transform: translateY(0);
		}
		30% {
			opacity: 1;
			transform: translateY(-2px);
		}
	}

	.typing-dot {
		animation: typing-dot 1.2s ease-in-out infinite;
	}

	@media (prefers-reduced-motion: reduce) {
		.typing-dot {
			animation: none;
			opacity: 0.6;
		}
	}
</style>

<script lang="ts">
	import { BOARD_STATUS_VALUES } from './task-board-model';
	import { taskBoardViewportHeight } from './task-board-viewport-height';
	import './task-board-layout.css';
	let { label, statusLabel }: { label: string; statusLabel: (status: string) => string } = $props();
</script>

<div role="status" aria-live="polite" aria-busy="true" data-task-skeleton class="sticky top-0 -mx-4 min-w-0 bg-background md:-mx-8">
	<span class="sr-only">{label}</span>
	<div data-task-board-scroll class="h-[var(--task-board-height,32rem)] min-h-80 min-w-0 overflow-x-auto overflow-y-hidden px-4 pb-2 md:px-8" use:taskBoardViewportHeight>
		<div class="flex h-full min-w-max gap-3 pr-4 md:pr-8" aria-hidden="true">
			{#each BOARD_STATUS_VALUES as status}
				<section class="task-board-column flex h-full min-h-0 shrink-0 flex-col overflow-hidden rounded-lg border bg-muted/30">
					<header class="flex h-11 items-center gap-2 border-b bg-card px-3">
						<h3 class="text-sm font-semibold">{statusLabel(status)}</h3>
						<span class="h-4 w-6 rounded bg-muted motion-safe:animate-pulse"></span>
					</header>
					<div class="flex flex-col gap-3 p-2.5">
						{#each [0, 1, 2] as row}
							<div class="flex h-28 flex-col gap-3 rounded-md border bg-card p-3 motion-safe:animate-pulse">
								<div class={row % 2 ? 'h-3 w-2/3 rounded bg-muted' : 'h-3 w-5/6 rounded bg-muted'}></div>
								<div class="h-3 w-1/2 rounded bg-muted"></div>
								<div class="mt-auto flex gap-2"><span class="size-5 rounded-full bg-muted"></span><span class="h-4 w-16 rounded bg-muted"></span></div>
							</div>
						{/each}
					</div>
				</section>
			{/each}
		</div>
	</div>
</div>

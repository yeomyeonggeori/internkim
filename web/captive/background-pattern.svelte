<script lang="ts">
	import Calendar from '@lucide/svelte/icons/calendar';
	import CircleCheck from '@lucide/svelte/icons/circle-check';
	import MessageCircle from '@lucide/svelte/icons/message-circle';
	import Sprout from '@lucide/svelte/icons/sprout';
	import Wifi from '@lucide/svelte/icons/wifi';

	const tileSize = 180;
	const placements = [
		{ icon: Wifi, x: 12, y: 14, size: 31, angle: -10 },
		{ icon: MessageCircle, x: 104, y: 8, size: 29, angle: 8 },
		{ icon: Calendar, x: 58, y: 70, size: 28, angle: -6 },
		{ icon: Sprout, x: 132, y: 80, size: 31, angle: 10 },
		{ icon: CircleCheck, x: 10, y: 118, size: 26, angle: 6 },
		{ icon: Wifi, x: 96, y: 130, size: 24, angle: 14 }
	];
</script>

<svg class="pointer-events-none fixed inset-0 size-full" aria-hidden="true">
	<defs>
		<pattern id="captive-pattern" width={tileSize} height={tileSize} patternUnits="userSpaceOnUse">
			{#each placements as placement, index (index)}
				<g transform="rotate({placement.angle} {placement.x + placement.size / 2} {placement.y + placement.size / 2})">
					<placement.icon
						x={placement.x}
						y={placement.y}
						size={placement.size}
						color="white"
						strokeWidth={1.6}
						opacity={0.16}
					/>
				</g>
			{/each}
		</pattern>
	</defs>
	<rect width="100%" height="100%" fill="url(#captive-pattern)" />
</svg>

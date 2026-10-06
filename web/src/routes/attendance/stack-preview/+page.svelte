<script lang="ts">
import ColoredOutlineBadge from '$lib/components/colored-outline-badge.svelte';
import DefinitionBadge from '$lib/components/definition-badge.svelte';
import LocationLabel from '../shared/location-label.svelte';
import PersonAvatarStack from '$lib/components/person-avatar-stack.svelte';
import { attendancePreviewEnabled } from '$lib/attendance/attendance-preview';
const people = Array.from({length: 127}, (_,i) => ({name: '샘플'+i, seed:'sample-stack-'+i}));
const sizes = [
 { name: 'small', avatarClass: 'size-5 ring-2 ring-background' },
 { name: 'default', avatarClass: undefined },
 { name: 'large', avatarClass: 'size-10 ring-2 ring-background' }
];
</script>
{#if attendancePreviewEnabled()}
<div class="flex flex-col gap-8 p-6">
 <h1>공용 Avatar Group 검사</h1>
 <div data-testid="stack-one"><PersonAvatarStack people={people.slice(0,5)} max={4} /></div>
 <div data-testid="stack-many"><PersonAvatarStack people={people.slice(0,16)} max={4} /></div>
 <div data-testid="stack-none"><PersonAvatarStack people={people.slice(0,3)} max={3} /></div>
 <div class="grid gap-4" data-testid="stack-sizing">
  {#each sizes as size (size.name)}
   {#each [1, 12, 123] as remaining (remaining)}
    <div class="flex items-center gap-3" data-testid={`stack-sizing-${size.name}-${remaining}`}>
     <span class="w-20 shrink-0 text-xs">{size.name} +{remaining}</span>
     <PersonAvatarStack people={people.slice(0,4 + remaining)} max={4} avatarClass={size.avatarClass} />
    </div>
   {/each}
  {/each}
 </div>
 <div class="flex flex-wrap gap-3" data-testid="color-tests"><ColoredOutlineBadge color="#fff3a0">밝은 노랑</ColoredOutlineBadge><ColoredOutlineBadge color="#08154a">짙은 파랑</ColoredOutlineBadge><ColoredOutlineBadge>색 없음</ColoredOutlineBadge><DefinitionBadge label="업무 종류" color="#ef8060" /><LocationLabel name="등록된 근무지" color="#008080" /></div>
</div>
{/if}

import type { MessengerReadCount } from './messenger-api';

const largestShownCount = 99;

export function unreadBadgeLabel(unread: MessengerReadCount | undefined): string {
	if (!unread || unread.status === 'unknown' || unread.value <= 0) return '';
	if (unread.value > largestShownCount) return `${largestShownCount}+`;
	return unread.status === 'at_least' ? `${unread.value}+` : String(unread.value);
}

export function isUnreadEmphasized(unread: MessengerReadCount | undefined, isMuted: boolean): boolean {
	return !isMuted && unreadBadgeLabel(unread) !== '';
}

export function conversationNameClass(unread: MessengerReadCount | undefined, isMuted: boolean, isActive: boolean): string {
	if (isUnreadEmphasized(unread, isMuted)) return 'font-bold';
	if (isActive) return '';
	return 'text-sidebar-foreground/70';
}

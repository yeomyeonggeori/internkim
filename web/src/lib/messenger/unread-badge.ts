const largestShownCount = 99;

export function unreadBadgeLabel(unreadCount: number | undefined): string {
	if (!unreadCount || unreadCount <= 0) return '';
	return unreadCount > largestShownCount ? `${largestShownCount}+` : String(unreadCount);
}

export function isUnreadEmphasized(unreadCount: number | undefined, isMuted: boolean): boolean {
	return !isMuted && (unreadCount ?? 0) > 0;
}

export function conversationNameClass(unreadCount: number | undefined, isMuted: boolean, isActive: boolean): string {
	if (isUnreadEmphasized(unreadCount, isMuted)) return 'font-bold';
	if (isActive) return '';
	return 'text-sidebar-foreground/70';
}

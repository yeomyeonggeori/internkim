import BrainIcon from '@lucide/svelte/icons/brain';
import CalendarIcon from '@lucide/svelte/icons/calendar';
import FlameIcon from '@lucide/svelte/icons/flame';
import FolderOpenIcon from '@lucide/svelte/icons/folder-open';
import HeartHandshakeIcon from '@lucide/svelte/icons/heart-handshake';
import LibraryBigIcon from '@lucide/svelte/icons/library-big';
import MailIcon from '@lucide/svelte/icons/mail';
import MessageCircleIcon from '@lucide/svelte/icons/message-circle';
import NetworkIcon from '@lucide/svelte/icons/network';
import SquareCheckBigIcon from '@lucide/svelte/icons/square-check-big';

export const appSectionIcons = {
	messenger: MessageCircleIcon,
	task: SquareCheckBigIcon,
	calendar: CalendarIcon,
	mail: MailIcon,
	attendance: FlameIcon,
	organization: NetworkIcon,
	crm: HeartHandshakeIcon,
	dataRoom: LibraryBigIcon,
	memory: BrainIcon,
	files: FolderOpenIcon
} as const;

export type AppSection = keyof typeof appSectionIcons;

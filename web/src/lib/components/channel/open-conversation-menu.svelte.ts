class OpenConversationMenu {
	menuID = $state<string | null>(null);

	isOpen(menuID: string): boolean {
		return this.menuID === menuID;
	}

	setOpen(menuID: string, isOpen: boolean): void {
		if (isOpen) {
			this.menuID = menuID;
			return;
		}
		if (this.menuID === menuID) this.menuID = null;
	}
}

export const openConversationMenu = new OpenConversationMenu();

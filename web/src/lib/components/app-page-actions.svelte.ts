export type PageRefreshHandler = () => void | Promise<void>;

class PageActions {
	refreshHandler = $state<PageRefreshHandler | null>(null);
	isRefreshing = $state(false);

	setRefresh = (handler: PageRefreshHandler) => {
		this.refreshHandler = handler;
		return () => {
			if (this.refreshHandler !== handler) return;
			this.refreshHandler = null;
			this.isRefreshing = false;
		};
	};

	refresh = async () => {
		if (this.isRefreshing) return;
		const handler = this.refreshHandler;
		if (!handler) {
			location.reload();
			return;
		}
		this.isRefreshing = true;
		try {
			await handler();
		} finally {
			this.isRefreshing = false;
		}
	};
}

export const pageActions = new PageActions();

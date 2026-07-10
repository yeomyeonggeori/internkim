(function initializeInternKimBoardsPlugin(globalScope) {
	const pluginID = 'com.internkim.ephemeral';
	const boardViews = [
		{ key: 'flow', label: '업무', path: '/flow/' },
		{ key: 'calendar', label: '일정', path: '/calendar/' },
		{ key: 'attendance', label: '근태', path: '/attendance/' },
	];

	function createBoardPanel(React) {
		const panelStyle = {
			display: 'flex',
			flexDirection: 'column',
			height: '100%',
			backgroundColor: 'var(--center-channel-bg)',
		};
		const tabBarStyle = {
			display: 'flex',
			alignItems: 'center',
			gap: '4px',
			padding: '8px 12px',
			borderBottom: '1px solid rgba(var(--center-channel-color-rgb), 0.12)',
		};
		const frameContainerStyle = { flex: 1, minHeight: 0 };

		function tabStyle(isActive) {
			return {
				border: 'none',
				borderRadius: '4px',
				padding: '6px 12px',
				fontSize: '13px',
				fontWeight: 600,
				cursor: 'pointer',
				backgroundColor: isActive ? 'var(--button-bg)' : 'transparent',
				color: isActive ? 'var(--button-color)' : 'rgba(var(--center-channel-color-rgb), 0.72)',
			};
		}

		function frameStyle(isActive) {
			return {
				width: '100%',
				height: '100%',
				border: 'none',
				display: isActive ? 'block' : 'none',
			};
		}

		const openInBrowserStyle = {
			marginLeft: 'auto',
			display: 'inline-flex',
			alignItems: 'center',
			padding: '6px',
			borderRadius: '4px',
			color: 'rgba(var(--center-channel-color-rgb), 0.72)',
		};

		const openInBrowserIcon = React.createElement(
			'svg',
			{ width: 16, height: 16, viewBox: '0 0 24 24', fill: 'currentColor' },
			React.createElement('path', {
				d: 'M14 3h7v7h-2V6.41l-9.29 9.3-1.42-1.42 9.3-9.29H14V3zM5 5h6v2H7v10h10v-4h2v6H5V5z',
			}),
		);

		return function BoardPanel() {
			const initialViewKey = boardViews[0].key;
			const activeViewState = React.useState(initialViewKey);
			const activeViewKey = activeViewState[0];
			const setActiveViewKey = activeViewState[1];
			const visitedViewsState = React.useState([initialViewKey]);
			const visitedViewKeys = visitedViewsState[0];
			const setVisitedViewKeys = visitedViewsState[1];

			function activateView(viewKey) {
				setActiveViewKey(viewKey);
				if (visitedViewKeys.indexOf(viewKey) < 0) {
					setVisitedViewKeys(visitedViewKeys.concat(viewKey));
				}
			}

			const activeView = boardViews.filter(function matchesActiveKey(view) {
				return view.key === activeViewKey;
			})[0];

			const tabs = boardViews.map(function renderTab(view) {
				return React.createElement(
					'button',
					{
						key: view.key,
						style: tabStyle(view.key === activeViewKey),
						onClick: function selectView() {
							activateView(view.key);
						},
					},
					view.label,
				);
			});

			const openInBrowserLink = React.createElement(
				'a',
				{
					key: 'open-in-browser',
					href: activeView.path,
					target: '_blank',
					rel: 'noopener noreferrer',
					style: openInBrowserStyle,
					'aria-label': '브라우저에서 열기',
					title: '브라우저에서 열기',
				},
				openInBrowserIcon,
			);
			const frames = boardViews
				.filter(function isVisited(view) {
					return visitedViewKeys.indexOf(view.key) >= 0;
				})
				.map(function renderFrame(view) {
					return React.createElement('iframe', {
						key: view.key,
						src: view.path,
						title: view.label,
						style: frameStyle(view.key === activeViewKey),
					});
				});

			return React.createElement(
				'div',
				{ style: panelStyle },
				React.createElement('div', { style: tabBarStyle }, tabs.concat([openInBrowserLink])),
				React.createElement('div', { style: frameContainerStyle }, frames),
			);
		};
	}

	function createChannelHeaderIcon(React) {
		return React.createElement(
			'svg',
			{ width: 18, height: 18, viewBox: '0 0 24 24', fill: 'currentColor' },
			React.createElement('rect', { x: 3, y: 3, width: 5, height: 18, rx: 1 }),
			React.createElement('rect', { x: 10, y: 3, width: 5, height: 12, rx: 1 }),
			React.createElement('rect', { x: 17, y: 3, width: 5, height: 8, rx: 1 }),
		);
	}

	function InternKimBoardsPlugin() {}

	InternKimBoardsPlugin.prototype.initialize = function initialize(registry, store) {
		const React = globalScope.React;
		const BoardPanel = createBoardPanel(React);
		const rhsRegistration = registry.registerRightHandSidebarComponent(BoardPanel, '김인턴');
		registry.registerChannelHeaderButtonAction(
			createChannelHeaderIcon(React),
			function toggleBoardPanel() {
				store.dispatch(rhsRegistration.toggleRHSPlugin);
			},
			'김인턴',
			'업무 · 일정 · 근태 보기',
		);
	};

	globalScope.registerPlugin(pluginID, new InternKimBoardsPlugin());
})(window);

import { describe, expect, test } from 'bun:test';

function createElement(type, props, ...children) {
	return { type, props: props || {}, children: children.flat() };
}

let hookStates = [];
let hookIndex = 0;

function useState(initialValue) {
	const stateIndex = hookIndex;
	hookIndex += 1;
	if (hookStates.length <= stateIndex) {
		hookStates.push(initialValue);
	}
	function setState(nextValue) {
		hookStates[stateIndex] = nextValue;
	}
	return [hookStates[stateIndex], setState];
}

function renderComponent(Component) {
	hookIndex = 0;
	return Component();
}

function resetHookStates() {
	hookStates = [];
	hookIndex = 0;
}

function collectElements(node, predicate, results = []) {
	if (!node || typeof node !== 'object') {
		return results;
	}
	if (predicate(node)) {
		results.push(node);
	}
	for (const child of node.children || []) {
		collectElements(child, predicate, results);
	}
	return results;
}

const pluginRegistration = {};
globalThis.window = {
	React: { createElement, useState },
	location: {
		protocol: 'https:',
		hostname: 'poc-0.intern.kim',
		pathname: '/tenant01/channels/town-square',
	},
	registerPlugin(id, plugin) {
		pluginRegistration.id = id;
		pluginRegistration.plugin = plugin;
	},
};
await import('./main.js');

function initializePlugin() {
	const registered = {};
	const registry = {
		registerRightHandSidebarComponent(component, title) {
			registered.sidebar = { component, title };
			return { toggleRHSPlugin: { type: 'TOGGLE_INTERNKIM_RHS' } };
		},
		registerChannelHeaderButtonAction(icon, action, dropdownText, tooltipText) {
			registered.headerButton = { icon, action, dropdownText, tooltipText };
		},
		registerNeedsTeamRoute(route, component) {
			registered.teamRoute = { route, component };
		},
	};
	const dispatchedActions = [];
	const store = {
		dispatch(action) {
			dispatchedActions.push(action);
		},
	};
	pluginRegistration.plugin.initialize(registry, store);
	return { registered, dispatchedActions };
}

describe('internkim mattermost webapp plugin', () => {
	test('registers the plugin with the manifest id', () => {
		expect(pluginRegistration.id).toBe('com.internkim.ephemeral');
		expect(typeof pluginRegistration.plugin.initialize).toBe('function');
	});

	test('registers the sidebar and a header button that toggles it', () => {
		const { registered, dispatchedActions } = initializePlugin();
		expect(registered.sidebar.title).toBe('김인턴');
		expect(typeof registered.sidebar.component).toBe('function');
		expect(registered.headerButton.dropdownText).toBe('김인턴');
		registered.headerButton.action();
		expect(dispatchedActions).toEqual([{ type: 'TOGGLE_INTERNKIM_RHS' }]);
	});

	test('renders tabs for flow, calendar, and attendance with only the flow frame mounted', () => {
		resetHookStates();
		const { registered } = initializePlugin();
		const tree = renderComponent(registered.sidebar.component);
		const tabLabels = collectElements(tree, (node) => node.type === 'button').map(
			(node) => node.children[0],
		);
		expect(tabLabels).toEqual(['업무', '일정', '근태']);
		const frames = collectElements(tree, (node) => node.type === 'iframe');
		expect(frames.map((frame) => frame.props.src)).toEqual([
			'https://poc0-t01.intern.kim/auth/mattermost/start?return=%2Fflow%2F',
		]);
		expect(frames[0].props.style.display).toBe('block');
	});

	test('selecting the calendar tab mounts its frame and hides the flow frame', () => {
		resetHookStates();
		const { registered } = initializePlugin();
		const firstTree = renderComponent(registered.sidebar.component);
		const calendarTab = collectElements(
			firstTree,
			(node) => node.type === 'button' && node.children[0] === '일정',
		)[0];
		calendarTab.props.onClick();
		const secondTree = renderComponent(registered.sidebar.component);
		const framesBySource = {};
		for (const frame of collectElements(secondTree, (node) => node.type === 'iframe')) {
			framesBySource[frame.props.src] = frame.props.style.display;
		}
		expect(framesBySource).toEqual({
			'https://poc0-t01.intern.kim/auth/mattermost/start?return=%2Fflow%2F': 'none',
			'https://poc0-t01.intern.kim/auth/mattermost/start?return=%2Fcalendar%2F': 'block',
		});
		const links = collectElements(secondTree, (node) => node.type === 'a');
		expect(links.map((link) => link.props.href)).toEqual(['https://poc0-t01.intern.kim/calendar/']);
	});

	test('unmounts the calendar frame while another tab is active', () => {
		resetHookStates();
		const { registered } = initializePlugin();
		const firstTree = renderComponent(registered.sidebar.component);
		const calendarTab = collectElements(
			firstTree,
			(node) => node.type === 'button' && node.children[0] === '일정',
		)[0];
		calendarTab.props.onClick();
		const secondTree = renderComponent(registered.sidebar.component);
		const flowTab = collectElements(
			secondTree,
			(node) => node.type === 'button' && node.children[0] === '업무',
		)[0];
		flowTab.props.onClick();
		const thirdTree = renderComponent(registered.sidebar.component);
		const frameSources = collectElements(thirdTree, (node) => node.type === 'iframe').map(
			(frame) => frame.props.src,
		);

		expect(frameSources).toEqual([
			'https://poc0-t01.intern.kim/auth/mattermost/start?return=%2Fflow%2F',
		]);
	});

	test('links only to the browser view for the active tab', () => {
		resetHookStates();
		const { registered } = initializePlugin();
		const tree = renderComponent(registered.sidebar.component);
		const links = collectElements(tree, (node) => node.type === 'a');
		expect(links.map((link) => link.props.href)).toEqual(['https://poc0-t01.intern.kim/flow/']);
		expect(links[0].props.target).toBe('_blank');
	});

	test('keeps relative board URLs outside the PoC tenant route', () => {
		resetHookStates();
		window.location.pathname = '/login';
		const { registered } = initializePlugin();
		const tree = renderComponent(registered.sidebar.component);
		const frame = collectElements(tree, (node) => node.type === 'iframe')[0];
		expect(frame.props.src).toBe('/flow/');
		window.location.pathname = '/tenant01/channels/town-square';
	});

	test('does not register a fullscreen team route', () => {
		const { registered } = initializePlugin();
		expect(registered.teamRoute).toBeUndefined();
	});
});

import { getCurrentWindow } from '@tauri-apps/api/window';
import { mount } from 'svelte';
import App from './App.svelte';
import HandoffOverlay from './HandoffOverlay.svelte';
import './style.css';

const isHandoffOverlayWindow = getCurrentWindow().label === 'browser-handoff-overlay';
const rootComponent = isHandoffOverlayWindow ? HandoffOverlay : App;

const app = mount(rootComponent, {
	target: document.getElementById('app') as HTMLElement
});

export default app;

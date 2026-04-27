import App from './App.svelte';
import './style.css';
import { mount } from 'svelte';

const app = mount(App, {
	target: document.getElementById('app') as HTMLElement
});

export default app;

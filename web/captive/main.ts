import { mount } from 'svelte';
import './captive.css';
import CaptivePage from './captive-page.svelte';

const target = document.getElementById('captive');
if (!target) {
	throw new Error('The captive page has no #captive element to mount into.');
}
mount(CaptivePage, { target });

import { createApp } from 'vue';
import { createPinia } from 'pinia';
import PrimeVue from 'primevue/config';

import './styles/reset.css';
import './styles/theme.css';
import './echarts';

import App from './App.vue';
import { router } from './router';
import { useThemeStore } from './stores/theme';

const app = createApp(App);
const pinia = createPinia();

app.use(pinia);
app.use(router);
// PrimeVue 4: unstyled passthrough — components ship behaviour + a11y,
// we own all visuals via theme.css + scoped component CSS.
app.use(PrimeVue, { unstyled: true });

// Pull the theme store once so its watchEffect mounts before paint
// and `<html data-theme>` is set on the first frame.
useThemeStore();

app.mount('#app');

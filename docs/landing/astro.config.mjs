import { defineConfig } from 'astro/config';
import tailwindcss from '@tailwindcss/vite';

// Builds only the docs home page; netlify.toml copies it over MkDocs' index.html.
export default defineConfig({
  site: 'https://kamaji.clastix.io',
  devToolbar: { enabled: false },
  vite: { plugins: [tailwindcss()] },
});

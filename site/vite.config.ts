import { defineConfig } from 'vite'
import { resolve } from 'node:path'
import react from '@vitejs/plugin-react'

// GitHub Pages project-site path for repo BarterX-Tech/dossierx.
// Read from VITE_BASE so local preview can override; fall back to '/dossierx/'.
export default defineConfig(() => ({
  base: process.env.VITE_BASE ?? '/dossierx/',
  plugins: [react()],
  build: {
    // Two genuine static entry points (not a client-side route): the site
    // deploys as static GitHub Pages output with no SPA rewrite, so the full
    // release history lives at its own real releases.html rather than behind
    // a router that would 404 on a hard refresh.
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'index.html'),
        releases: resolve(__dirname, 'releases.html'),
      },
    },
  },
}))

import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// GitHub Pages project-site path for repo BarterX-Tech/dossierx.
// Read from VITE_BASE so local preview can override; fall back to '/dossierx/'.
export default defineConfig(() => ({
  base: process.env.VITE_BASE ?? '/dossierx/',
  plugins: [react()],
}))

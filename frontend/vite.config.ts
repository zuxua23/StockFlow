import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  test: { environment: 'node' },
  server: {
    proxy: {
      '/api':'http://localhost:8080'}},
})





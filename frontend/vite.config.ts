import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0', // Required for Docker container mapping
    port: 5173,
    watch: {
      usePolling: true, // Crucial for hot-reloading on Windows hosts
    },
  },
});

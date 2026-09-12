// admin-portal/vite.config.ts
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0', // Required for Docker container mapping
    port: 5174,     // Separated port for Administration Portal
    watch: {
      usePolling: true, // Crucial for hot-reloading on Windows hosts
    },
  },
});
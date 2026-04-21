/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
export default defineConfig(function () {
    var apiTarget = process.env.VITE_API_PROXY_TARGET || process.env.VITE_API_BASE_URL || 'http://localhost:8080';
    return {
        plugins: [react(), tailwindcss()],
        server: {
            proxy: {
                '/api': {
                    target: apiTarget,
                    changeOrigin: true,
                },
                '/healthz': {
                    target: apiTarget,
                    changeOrigin: true,
                },
            },
        },
        test: {
            environment: 'jsdom',
            setupFiles: './src/test/setup.ts',
            globals: true,
        },
    };
});

import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  return {
    plugins: [react()],
    resolve: { alias: { '@': path.resolve(__dirname, 'src') } },
    esbuild: {
      drop: mode === 'production' ? ['console', 'debugger'] : [],
    },
    server: {
      port: Number(env.VITE_PORT || 5173),
      proxy: {
        '/api': {
          target: env.VITE_API_BASE || 'http://localhost:8080',
          changeOrigin: true,
        },
        '/ws': {
          target: env.VITE_API_BASE || 'http://localhost:8080',
          ws: true,
          changeOrigin: true,
        },
      },
    },
    build: {
      sourcemap: mode !== 'production',
      rollupOptions: {
        output: {
          manualChunks: {
            vendor: ['react', 'react-dom', 'react-router-dom'],
            antd: ['antd'],
            reactflow: ['reactflow', '@reactflow/background', '@reactflow/controls', '@reactflow/minimap'],
          },
        },
      },
    },
  };
});

import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: {
        name: 'Открывай — мир путешествий',
        short_name: 'Открывай',
        description: 'Путешествия возвращают миру цвет',
        theme_color: '#0d0b68',
        background_color: '#edefff',
        display: 'standalone',
        lang: 'ru',
        icons: [
          { src: '/icon.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any maskable' },
          { src: '/apple-touch-icon.png', sizes: '180x180', type: 'image/png', purpose: 'any' }
        ]
      }
    })
  ],
  server: {
    port: 5173,
    proxy: { '/api': 'http://localhost:8080' }
  }
})


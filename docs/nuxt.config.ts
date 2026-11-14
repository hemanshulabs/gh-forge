export default defineNuxtConfig({
  extends: ['docus'],
  css: ['~/assets/css/main.css'],
  app: {
    head: {
      title: 'Forge - A Rich Terminal UI for GitHub',
      link: [
        { rel: 'icon', type: 'image/png', href: '/favicon.png' },
        { rel: 'shortcut icon', href: '/favicon.ico' },
        { rel: 'apple-touch-icon', href: '/favicon.png' }
      ],
      meta: [
        { name: 'description', content: 'A rich terminal UI for GitHub that doesn\'t break your flow.' },
        { property: 'og:image', content: '/og.png' },
        { property: 'og:title', content: 'Forge - A Rich Terminal UI for GitHub' },
        { property: 'og:description', content: 'A rich terminal UI for GitHub that doesn\'t break your flow.' },
        { name: 'twitter:card', content: 'summary_large_image' },
        { name: 'theme-color', content: '#00F9FB' }
      ]
    }
  }
})

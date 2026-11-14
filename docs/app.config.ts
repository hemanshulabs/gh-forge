export default defineAppConfig({
  docus: {
    locale: 'en',
    title: 'Forge',
    description: 'A rich terminal UI for GitHub that doesn\'t break your flow.',
    image: '/og.png',
  },
  header: {
    title: 'Forge',
    logo: {
      light: '/logo-header-light.png',
      dark: '/logo-header.png',
      alt: 'Forge',
      favicon: '/favicon.png',
      class: 'h-7 sm:h-8 w-auto'
    }
  },
  seo: {
    title: 'Forge',
    titleTemplate: '%s - Forge',
    description: 'A rich terminal UI for GitHub that doesn\'t break your flow.'
  },
  github: {
    url: 'https://github.com/hemanshulabs/gh-forge'
  },
  socials: {
    github: 'https://github.com/hemanshulabs/gh-forge'
  },
  ui: {
    colors: {
      primary: 'cyan',
      neutral: 'slate',
    }
  }
})

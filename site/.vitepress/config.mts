import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "ESG Award System",
  description: "The official documentation site of ESG Award Contest",
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Audit Log', link: '/audit-log/' },
    ],

    sidebar: [
      {
        text: 'Audit Log',
        items: [
          { text: 'Concepts', link: '/audit-log/' },
          { text: 'Architecture', link: '/audit-log/architecture' },
          { text: 'Implementation', link: '/audit-log/implementation' },
          { text: 'Decision Thinking', link: '/audit-log/decisions' },
          { text: 'Technical FAQ', link: '/audit-log/faq' }
        ]
      }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/vuejs/vitepress' }
    ]
  }
})

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
      { text: 'File Upload', link: '/file-upload/' },
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
      },
      {
        text: 'File Upload',
        items: [
          { text: 'Concepts', link: '/file-upload/' },
          { text: 'Architecture', link: '/file-upload/architecture' },
          { text: 'Implementation', link: '/file-upload/implementation' },
          { text: 'Decision Thinking', link: '/file-upload/decisions' },
          { text: 'Technical FAQ', link: '/file-upload/faq' }
        ]
      }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/vuejs/vitepress' }
    ]
  }
})

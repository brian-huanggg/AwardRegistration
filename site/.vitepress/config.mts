import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
const moduleSidebar = [
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
  },
  {
    text: 'Authentication',
    items: [
      { text: 'Concepts', link: '/authentication/' },
      { text: 'Architecture', link: '/authentication/architecture' },
      { text: 'Implementation', link: '/authentication/implementation' },
      { text: 'Decision Thinking', link: '/authentication/decisions' },
      { text: 'Technical FAQ', link: '/authentication/faq' }
    ]
  },
  {
    text: 'Authorization & Uniform 404',
    items: [
      { text: 'Concepts', link: '/authorization/' },
      { text: 'Architecture', link: '/authorization/architecture' },
      { text: 'Implementation', link: '/authorization/implementation' },
      { text: 'Decision Thinking', link: '/authorization/decisions' },
      { text: 'Technical FAQ', link: '/authorization/faq' }
    ]
  },
  {
    text: 'Team & Join Request',
    items: [
      { text: 'Concepts', link: '/team/' },
      { text: 'Architecture', link: '/team/architecture' },
      { text: 'Implementation', link: '/team/implementation' },
      { text: 'Decision Thinking', link: '/team/decisions' },
      { text: 'Technical FAQ', link: '/team/faq' }
    ]
  },
  {
    text: 'Transactions & Consistency',
    items: [
      { text: 'Concepts', link: '/transactions/' },
      { text: 'Architecture', link: '/transactions/architecture' },
      { text: 'Implementation', link: '/transactions/implementation' },
      { text: 'Decision Thinking', link: '/transactions/decisions' },
      { text: 'Technical FAQ', link: '/transactions/faq' }
    ]
  }
]

export default defineConfig({
  title: "ESG Award System",
  description: "The official documentation site of ESG Award Contest",
  // Maintainer index for the interview bank; kept in the repo, not rendered.
  srcExclude: ['Interview/README.md'],
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: 'Home', link: '/' },
      {
        text: 'Module',
        activeMatch: '^/(audit-log|file-upload|authentication|authorization|team|transactions)/',
        items: [
          { text: 'Audit Log', link: '/audit-log/' },
          { text: 'File Upload', link: '/file-upload/' },
          { text: 'Authentication', link: '/authentication/' },
          { text: 'Authorization', link: '/authorization/' },
          { text: 'Team', link: '/team/' },
          { text: 'Transactions', link: '/transactions/' }
        ]
      },
      { text: 'Interview', link: '/Interview/Rubric-template', activeMatch: '^/Interview/' }
    ],

    // Modules share one sidebar; Interview has its own.
    sidebar: {
      '/Interview/': [
        {
          text: 'Interview',
          items: [
            { text: 'Question Template', link: '/Interview/Rubric-template' }
          ]
        },
        {
          text: 'Modules',
          items: [
            { text: 'Authentication & Session', link: '/Interview/modules/auth-session' },
            { text: 'Account', link: '/Interview/modules/account' },
            { text: 'Idea', link: '/Interview/modules/idea' },
            { text: 'Storage', link: '/Interview/modules/storage' },
            { text: 'Deadline', link: '/Interview/modules/deadline' },
            { text: 'Audit', link: '/Interview/modules/audit' }
          ]
        }
      ],
      '/audit-log/': moduleSidebar,
      '/file-upload/': moduleSidebar,
      '/authentication/': moduleSidebar,
      '/authorization/': moduleSidebar,
      '/team/': moduleSidebar,
      '/transactions/': moduleSidebar
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/vuejs/vitepress' }
    ]
  }
})

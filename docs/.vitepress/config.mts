// docs/.vitepress/config.mts
import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

// GitHub-compatible heading slugify so the canonical docs' GitHub-form anchors
// (e.g. "#62-roadmap") resolve identically on the site. Mirrors github-slugger:
// lower-case, strip punctuation except letters, numbers, connector `_` and
// hyphen, and map each space to a hyphen without collapsing runs.
function githubSlugify(str: string): string {
  return str
    .normalize('NFKC')
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\p{Pc}\- ]/gu, '')
    .replace(/ /g, '-')
}

export default withMermaid(defineConfig({
  title: 'pod-log-preserver',
  description: 'Preserve kubelet-rotated pod logs on EKS Auto Mode until a log agent has collected them',
  base: '/pod-log-preserver/',
  cleanUrls: true,
  markdown: {
    anchor: { slugify: githubSlugify },
  },
  srcExclude: ['superpowers/**'],
  // The canonical docs legitimately link to repo files outside the docs root
  // (CLAUDE.md, LICENSE); ignore only those specific outside-root targets so
  // real dead links still fail the build.
  ignoreDeadLinks: [
    /CLAUDE(\.md)?$/,
    /LICENSE$/,
  ],
  themeConfig: {
    search: { provider: 'local' },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/AkashiSN/pod-log-preserver' },
    ],
  },
  locales: {
    root: {
      label: 'English',
      lang: 'en',
      themeConfig: {
        nav: [
          { text: 'Getting Started', link: '/getting-started' },
          { text: 'Specification', link: '/specification/' },
          { text: 'Development', link: '/development/ci-cd' },
        ],
        sidebar: [
          { text: 'Overview', collapsed: false, items: [
            { text: 'Getting Started', link: '/getting-started' },
          ]},
          { text: 'Specification', collapsed: false, items: [
            { text: 'Contents', link: '/specification/' },
            { text: '1. Overview', link: '/specification/01-overview' },
            { text: '2. Scope', link: '/specification/02-scope' },
            { text: '3. Design', link: '/specification/03-design' },
            { text: '4. Operations', link: '/specification/04-operations' },
            { text: '5. Implementation', link: '/specification/05-implementation' },
            { text: '6. Release', link: '/specification/06-release' },
            { text: '7. Risks & Status', link: '/specification/07-risks' },
          ]},
          { text: 'Development', collapsed: false, items: [
            { text: 'CI/CD Design', link: '/development/ci-cd' },
            { text: 'Documentation Style', link: '/development/documentation-style' },
          ]},
        ],
      },
    },
    ja: {
      label: '日本語',
      lang: 'ja',
      link: '/ja/',
      themeConfig: {
        nav: [
          { text: 'はじめに', link: '/ja/getting-started' },
          { text: '仕様書', link: '/ja/specification/' },
          { text: '開発者向け', link: '/development/ci-cd' },
        ],
        sidebar: [
          { text: '概要', collapsed: false, items: [
            { text: 'はじめに', link: '/ja/getting-started' },
          ]},
          { text: '仕様書', collapsed: false, items: [
            { text: '目次', link: '/ja/specification/' },
            { text: '1. 概要', link: '/ja/specification/01-overview' },
            { text: '2. スコープ', link: '/ja/specification/02-scope' },
            { text: '3. 設計', link: '/ja/specification/03-design' },
            { text: '4. 運用', link: '/ja/specification/04-operations' },
            { text: '5. 実装', link: '/ja/specification/05-implementation' },
            { text: '6. リリース', link: '/ja/specification/06-release' },
            { text: '7. リスクと状況', link: '/ja/specification/07-risks' },
          ]},
          { text: '開発', collapsed: false, items: [
            { text: 'CI/CD 設計（英語）', link: '/development/ci-cd' },
            { text: 'ドキュメントスタイル（英語）', link: '/development/documentation-style' },
          ]},
        ],
      },
    },
  },
}))

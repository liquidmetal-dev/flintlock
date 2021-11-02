// @ts-check
// Note: type annotations allow type checking and IDEs autocompletion

const lightCodeTheme = require('prism-react-renderer/themes/github');
const darkCodeTheme = require('prism-react-renderer/themes/dracula');

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Flintlock',
  tagline: ' Lock, Stock, and Two Smoking MicroVMs. Create and manage the lifecycle of MicroVMs backed by containerd.',
  url: 'https://docs.flintlock.dev/',
  baseUrl: '/',
  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'warn',
  favicon: 'img/favicon.ico',
  organizationName: 'weaveworks',
  projectName: 'flintlock',
  trailingSlash: false,

  presets: [
    [
      '@docusaurus/preset-classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          sidebarPath: require.resolve('./sidebars.js'),
          // Please change this to your repo.
          editUrl: 'https://github.com/weaveworks/flintlock/edit/main/userdocs/',
        },
        blog: {
          showReadingTime: true,
          // Please change this to your repo.
          editUrl:
            'https://github.com/weaveworks/flintlock/edit/main/userdocs/blog/',
        },
        theme: {
          customCss: require.resolve('./src/css/custom.css'),
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      navbar: {
        title: 'Flintlock',
        logo: {
          alt: 'Flintlock Logo',
          src: 'img/logo.svg',
        },
        items: [
          {
            type: 'doc',
            docId: 'intro',
            position: 'left',
            label: 'Getting Started',
          },
          {
            type: 'doc',
            docId: 'proto',
            position: 'left',
            label: 'gRPC Proto',
          },
          {to: '/blog', label: 'Blog', position: 'left'},
          {
            href: 'https://github.com/weaveworks/flintlock',
            label: 'GitHub',
            position: 'right',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: 'Docs',
            items: [
              {
                label: 'Getting Started',
                to: '/docs/intro',
              },
              {
                label: 'gRPC Proto',
                to: '/docs/proto',
              },
              {
                label: 'ADR',
                to: 'https://github.com/weaveworks/flintlock/tree/main/docs/adr',
              },
            ],
          },
          {
            title: 'Community',
            items: [
              {
                label: 'Slack',
                href: 'https://slack.weave.works/',
              },
              {
                label: 'Twitter',
                href: 'https://twitter.com/weaveworks',
              },
            ],
          },
          {
            title: 'More',
            items: [
              {
                label: 'Blog',
                to: '/blog',
              },
              {
                label: 'GitHub',
                href: 'https://github.com/weaveworks/flintlock',
              },
            ],
          },
        ],
        copyright: `Copyright © ${new Date().getFullYear()} Weaveworks, Inc. Built with Docusaurus.`,
      },
      prism: {
        theme: lightCodeTheme,
        darkTheme: darkCodeTheme,
      },
    }),
};

module.exports = config;

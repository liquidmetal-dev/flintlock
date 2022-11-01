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
          editUrl: 'https://github.com/weaveworks-liquidmetal/flintlock/edit/main/userdocs/',
        },
        blog: {
          showReadingTime: true,
          // Please change this to your repo.
          editUrl:
            'https://github.com/weaveworks-liquidmetal/flintlock/edit/main/userdocs/blog/',
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
            label: 'Documentation',
          },
          {
            href: 'https://buf.build/weaveworks-liquidmetal/flintlock',
            position: 'left',
            label: 'gRPC Proto',
          },
          {
            href: '/flintlock-api',
            target: "_blank",
            position: 'left',
            label: 'HTTP API',
          },
          {
            href: 'https://github.com/weaveworks-liquidmetal/flintlock',
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
                label: 'Docs',
                to: '/docs/intro',
              },
              {
                label: 'gRPC Proto',
                href: 'https://buf.build/weaveworks-liquidmetal/flintlock',
              },
              {
                label: 'HTTP API',
                to: '/flintlock-api',
                target: '_blank',
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
                label: 'Liquid Metal',
                to: 'https://weaveworks-liquidmetal.github.io/site/',
              },
              {
                label: 'GitHub',
                href: 'https://github.com/weaveworks-liquidmetal/flintlock',
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

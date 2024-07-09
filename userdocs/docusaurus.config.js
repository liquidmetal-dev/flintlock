// @ts-check
// Note: type annotations allow type checking and IDEs autocompletion

const lightCodeTheme = require('prism-react-renderer/themes/github');
const darkCodeTheme = require('prism-react-renderer/themes/dracula');

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Flintlock',
  tagline: ' Lock, Stock, and Two Smoking MicroVMs. Create and manage the lifecycle of MicroVMs backed by containerd.',
  url: 'https://www.liquidmetal.dev',
  baseUrl: '/flintlock/',
  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'warn',
  favicon: 'img/favicon.ico',
  organizationName: 'liquidmetal-dev',
  projectName: 'flintlock',
  trailingSlash: true,

  presets: [
    [
      '@docusaurus/preset-classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          sidebarPath: require.resolve('./sidebars.js'),
          // Please change this to your repo.
          editUrl: 'https://github.com/liquidmetal-dev/flintlock/edit/main/userdocs/',
        },
        blog: {
          showReadingTime: true,
          // Please change this to your repo.
          editUrl:
            'https://github.com/liquidmetal-dev/flintlock/edit/main/userdocs/blog/',
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
            href: 'https://buf.build/liquidmetal-dev/flintlock',
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
            href: 'https://github.com/liquidmetal-dev/flintlock',
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
                href: 'https://buf.build/liquidmetal-dev/flintlock',
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
                to: 'https://www.liquidmetal.dev',
              },
              {
                label: 'GitHub',
                href: 'https://github.com/liquidmetal-dev/flintlock',
              },
            ],
          },
        ],
        copyright: `Copyright © ${new Date().getFullYear()} Liquid Metal Authors. Built with Docusaurus.`,
      },
      prism: {
        theme: lightCodeTheme,
        darkTheme: darkCodeTheme,
      },
    }),
};

module.exports = config;

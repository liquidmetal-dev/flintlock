/**
 * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */

// @ts-check

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  gettingStartedSidebar: [
    "intro",
    {
      type: 'category',
      label: 'Getting Started',
      link: {
        type: 'generated-index',
        description: 'A basic tutorial.',
      },
      items: [
        'getting-started/setup',
        'getting-started/network',
        'getting-started/containerd',
        'getting-started/firecracker',
        'getting-started/flintlock',
        'getting-started/usage',
      ],
    },
    {
      type: 'category',
      label: 'Advanced Guides',
      items: [
        'guides/images',
        'guides/metrics',
        'guides/service-opts',
        'guides/production',
      ],
    },
    {
      type: 'category',
      label: 'Troubleshooting',
      link: {
        type: 'generated-index',
        description: 'Help debugging common issues.',
      },
      items: [
        'troubleshooting/failed-to-reconcile-vmid',
      ],
    },
    {
      type: 'category',
      label: 'Development',
      items: [
        'development/intro',
      ],
    },
  ],
};

module.exports = sidebars;

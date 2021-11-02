import React from 'react';
import clsx from 'clsx';
import styles from './HomepageFeatures.module.css';

const FeatureList = [
  {
    title: 'What is Flintlock?',
    Svg: require('../../static/img/undraw_docusaurus_mountain.svg').default,
    description: (
      <>
        Flintlock is a service for creating and managing the lifecycle of
        microVMs on a host machine. Initially we will be supporting Firecracker. 
      </>
    ),
  },
  {
    title: 'Use Your Container Images',
    Svg: require('../../static/img/undraw_docusaurus_react.svg').default,
    description: (
      <>
        With Flintlock, you can use your container images for MicroVMs
        from OCI repositories, you don&apos;t have to create and deploy
        filesystem images on all your machines.
      </>
    ),
  },
  {
    title: 'Provision MicroVM on Demand',
    Svg: require('../../static/img/undraw_docusaurus_tree.svg').default,
    description: (
      <>
        Flintlock lets you focus on your deploying your application,
        and we&apos;ll do privision MicroVMs for you need.
      </>
    ),
  },
];

function Feature({Svg, title, description}) {
  return (
    <div className={clsx('col col--4')}>
      <div className="text--center">
        <Svg className={styles.featureSvg} alt={title} />
      </div>
      <div className="text--center padding-horiz--md">
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures() {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
